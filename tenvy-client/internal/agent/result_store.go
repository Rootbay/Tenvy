package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

type resultStoreConfig struct {
	Path      string
	Retention int
}

type resultStore struct {
	mu        sync.Mutex
	dir       string
	metaPath  string
	retention int
	nextID    uint64
	pending   int
}

type resultStoreMetadata struct {
	NextID  uint64 `json:"next_id"`
	Pending int    `json:"pending"`
}

func newResultStore(cfg resultStoreConfig) (*resultStore, error) {
	dir := strings.TrimSpace(cfg.Path)
	if dir == "" {
		return nil, errors.New("result store path must be provided")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create result store directory: %w", err)
	}

	store := &resultStore{
		dir:       dir,
		metaPath:  filepath.Join(dir, "meta.json"),
		retention: cfg.Retention,
	}

	if err := store.loadMetadata(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *resultStore) Append(result protocol.CommandResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.appendLocked(result); err != nil {
		return err
	}
	if err := s.enforceRetentionLocked(); err != nil {
		return err
	}
	return s.persistMetadataLocked()
}

func (s *resultStore) AppendAll(results []protocol.CommandResult) error {
	if len(results) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, result := range results {
		if err := s.appendLocked(result); err != nil {
			return err
		}
	}
	if err := s.enforceRetentionLocked(); err != nil {
		return err
	}
	return s.persistMetadataLocked()
}

func (s *resultStore) All() ([]protocol.CommandResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	names, err := s.sortedEntriesLocked()
	if err != nil {
		return nil, err
	}

	results := make([]protocol.CommandResult, 0, len(names))
	for _, name := range names {
		result, err := s.readEntryLocked(name)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func (s *resultStore) Tail(limit int) ([]protocol.CommandResult, error) {
	if limit <= 0 {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	names, err := s.sortedEntriesLocked()
	if err != nil {
		return nil, err
	}
	if limit < len(names) {
		names = names[len(names)-limit:]
	}

	results := make([]protocol.CommandResult, 0, len(names))
	for _, name := range names {
		result, err := s.readEntryLocked(name)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func (s *resultStore) RemoveFirst(count int) error {
	if count <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if count > s.pending {
		count = s.pending
	}
	if count == 0 {
		return nil
	}

	if err := s.deleteOldestLocked(count); err != nil {
		return err
	}
	return s.persistMetadataLocked()
}

func (s *resultStore) Count() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pending, nil
}

func (s *resultStore) appendLocked(result protocol.CommandResult) error {
	filename := filepath.Join(s.dir, formatResultFilename(s.nextID))
	s.nextID++
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	tmp, err := os.CreateTemp(s.dir, "result-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp result file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write result file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close result file: %w", err)
	}
	if err := os.Rename(tmpName, filename); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("finalize result file: %w", err)
	}
	s.pending++
	return nil
}

func (s *resultStore) enforceRetentionLocked() error {
	if s.retention <= 0 || s.pending <= s.retention {
		return nil
	}
	excess := s.pending - s.retention
	return s.deleteOldestLocked(excess)
}

func (s *resultStore) deleteOldestLocked(count int) error {
	if count <= 0 {
		return nil
	}
	names, err := s.sortedEntriesLocked()
	if err != nil {
		return err
	}
	if count > len(names) {
		count = len(names)
	}
	for i := 0; i < count; i++ {
		path := filepath.Join(s.dir, names[i])
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove result file: %w", err)
		}
		if s.pending > 0 {
			s.pending--
		}
	}
	return nil
}

func (s *resultStore) readEntryLocked(name string) (protocol.CommandResult, error) {
	path := filepath.Join(s.dir, name)
	file, err := os.Open(path)
	if err != nil {
		return protocol.CommandResult{}, fmt.Errorf("open result file: %w", err)
	}
	defer file.Close()

	var result protocol.CommandResult
	if err := json.NewDecoder(file).Decode(&result); err != nil {
		return protocol.CommandResult{}, fmt.Errorf("decode result file: %w", err)
	}
	return result, nil
}

func (s *resultStore) sortedEntriesLocked() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read result directory: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == filepath.Base(s.metaPath) {
			continue
		}
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		if _, err := parseResultFilename(name); err != nil {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (s *resultStore) loadMetadata() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(s.metaPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("open metadata: %w", err)
		}
		return s.rebuildMetadataLocked()
	}
	defer file.Close()

	var meta resultStoreMetadata
	if err := json.NewDecoder(file).Decode(&meta); err != nil {
		if rebuildErr := s.rebuildMetadataLocked(); rebuildErr != nil {
			return rebuildErr
		}
		return nil
	}

	s.nextID = meta.NextID
	s.pending = meta.Pending

	return s.reconcileMetadataLocked()
}

func (s *resultStore) rebuildMetadataLocked() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("scan result directory: %w", err)
	}

	var maxID uint64
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == filepath.Base(s.metaPath) || !strings.HasSuffix(name, ".json") {
			continue
		}
		id, err := parseResultFilename(name)
		if err != nil {
			continue
		}
		if id >= maxID {
			maxID = id + 1
		}
		count++
	}

	s.nextID = maxID
	s.pending = count
	return s.persistMetadataLocked()
}

func (s *resultStore) reconcileMetadataLocked() error {
	names, err := s.sortedEntriesLocked()
	if err != nil {
		return err
	}

	actualCount := len(names)
	if actualCount != s.pending {
		s.pending = actualCount
	}
	var maxID uint64
	for _, name := range names {
		id, err := parseResultFilename(name)
		if err != nil {
			continue
		}
		if id >= maxID {
			maxID = id + 1
		}
	}
	if maxID > s.nextID {
		s.nextID = maxID
	}
	if s.nextID == 0 {
		s.nextID = uint64(actualCount)
	}
	return s.persistMetadataLocked()
}

func (s *resultStore) persistMetadataLocked() error {
	meta := resultStoreMetadata{NextID: s.nextID, Pending: s.pending}
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	tmp, err := os.CreateTemp(s.dir, "meta-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp metadata: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write metadata: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close metadata: %w", err)
	}
	if err := os.Rename(tmpName, s.metaPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("replace metadata: %w", err)
	}
	return nil
}

func formatResultFilename(id uint64) string {
	return fmt.Sprintf("%020d.json", id)
}

func parseResultFilename(name string) (uint64, error) {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if base == "" {
		return 0, fmt.Errorf("invalid result filename: %s", name)
	}
	id, err := strconv.ParseUint(base, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid result filename: %s", name)
	}
	return id, nil
}

func defaultResultStorePath(pref BuildPreferences) string {
	installPath := strings.TrimSpace(pref.InstallPath)
	if installPath != "" {
		cleaned := filepath.Clean(installPath)
		info, err := os.Stat(cleaned)
		if err == nil && info.IsDir() {
			return filepath.Join(cleaned, "results")
		}
		return filepath.Join(filepath.Dir(cleaned), "results")
	}
	baseDir := dataDirectory(pref)
	return filepath.Join(baseDir, "results")
}
