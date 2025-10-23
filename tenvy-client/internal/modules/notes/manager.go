package notes

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

type Note struct {
	ID        string
	Title     string
	Body      string
	Shared    bool
	Version   int
	UpdatedAt time.Time
}

type noteContent struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type storedNote struct {
	ID         string    `json:"id"`
	Shared     bool      `json:"shared"`
	Version    int       `json:"version"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Ciphertext string    `json:"ciphertext"`
	Nonce      string    `json:"nonce"`
	Digest     string    `json:"digest"`
}

type noteFile struct {
	Notes []storedNote `json:"notes"`
}

type noteEnvelope struct {
	ID         string `json:"id"`
	Visibility string `json:"visibility"`
	UpdatedAt  string `json:"updatedAt"`
	Version    int    `json:"version"`
	Ciphertext string `json:"ciphertext"`
	Nonce      string `json:"nonce"`
	Digest     string `json:"digest"`
}

type noteSyncRequest struct {
	Notes []noteEnvelope `json:"notes"`
}

type noteSyncResponse struct {
	Notes []noteEnvelope `json:"notes"`
}

var (
	errNoteConflict = errors.New("note version conflict")
	errNoteTampered = errors.New("note integrity check failed")
)

type Manager struct {
	mu             sync.RWMutex
	path           string
	localKey       []byte
	sharedKey      []byte
	legacyLocalKey []byte
	notes          map[string]storedNote
	dirty          bool
	lastSync       time.Time
	syncInterval   time.Duration
}

func NewManager(path, localKeyMaterial, sharedKeyMaterial, legacyLocalKeyMaterial string) (*Manager, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("notes path is required")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create notes directory: %w", err)
	}

	manager := &Manager{
		path:         path,
		localKey:     deriveKey(localKeyMaterial),
		sharedKey:    deriveKey(sharedKeyMaterial),
		notes:        make(map[string]storedNote),
		syncInterval: 2 * time.Minute,
	}

	if strings.TrimSpace(legacyLocalKeyMaterial) != "" {
		manager.legacyLocalKey = deriveKey(legacyLocalKeyMaterial)
	}

	if err := manager.loadFromDisk(); err != nil {
		return nil, err
	}

	return manager, nil
}

func deriveKey(material string) []byte {
	sum := sha256.Sum256([]byte(material))
	return sum[:]
}

func (m *Manager) loadFromDisk() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	file, err := os.Open(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open notes file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("read notes file: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	var snapshot noteFile
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("decode notes file: %w", err)
	}

	for _, note := range snapshot.Notes {
		m.notes[note.ID] = note
	}

	return nil
}

func (m *Manager) persistLocked() error {
	snapshot := noteFile{Notes: make([]storedNote, 0, len(m.notes))}
	for _, note := range m.notes {
		snapshot.Notes = append(snapshot.Notes, note)
	}
	sort.Slice(snapshot.Notes, func(i, j int) bool {
		return snapshot.Notes[i].ID < snapshot.Notes[j].ID
	})

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode notes snapshot: %w", err)
	}

	tmpPath := m.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write temp notes file: %w", err)
	}

	if err := os.Rename(tmpPath, m.path); err != nil {
		return fmt.Errorf("replace notes file: %w", err)
	}

	return nil
}

func (m *Manager) keyFor(shared bool) []byte {
	if shared {
		return m.sharedKey
	}
	return m.localKey
}

func (m *Manager) decryptStoredLocked(stored storedNote) (noteContent, bool, error) {
	content, err := decryptNote(m.keyFor(stored.Shared), stored)
	if err == nil {
		return content, false, nil
	}

	if stored.Shared || len(m.legacyLocalKey) == 0 {
		return noteContent{}, false, err
	}

	legacyContent, legacyErr := decryptNote(m.legacyLocalKey, stored)
	if legacyErr != nil {
		return noteContent{}, false, err
	}

	ciphertext, nonce, digest, encErr := encryptNote(m.localKey, legacyContent)
	if encErr != nil {
		return noteContent{}, false, encErr
	}

	stored.Ciphertext = ciphertext
	stored.Nonce = nonce
	stored.Digest = digest
	m.notes[stored.ID] = stored

	return legacyContent, true, nil
}

func encryptNote(key []byte, content noteContent) (ciphertext, nonce, digest string, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", "", err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", "", err
	}

	nonceBytes := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", "", "", err
	}

	plaintext, err := json.Marshal(content)
	if err != nil {
		return "", "", "", err
	}

	cipherBytes := aead.Seal(nil, nonceBytes, plaintext, nil)
	digestBytes := sha256.Sum256(plaintext)

	return base64.StdEncoding.EncodeToString(cipherBytes), base64.StdEncoding.EncodeToString(nonceBytes), base64.StdEncoding.EncodeToString(digestBytes[:]), nil
}

func decryptNote(key []byte, stored storedNote) (noteContent, error) {
	var result noteContent

	cipherBytes, err := base64.StdEncoding.DecodeString(stored.Ciphertext)
	if err != nil {
		return result, fmt.Errorf("decode ciphertext: %w", err)
	}

	nonceBytes, err := base64.StdEncoding.DecodeString(stored.Nonce)
	if err != nil {
		return result, fmt.Errorf("decode nonce: %w", err)
	}

	digestBytes, err := base64.StdEncoding.DecodeString(stored.Digest)
	if err != nil {
		return result, fmt.Errorf("decode digest: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return result, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return result, err
	}

	plaintext, err := aead.Open(nil, nonceBytes, cipherBytes, nil)
	if err != nil {
		return result, err
	}

	checksum := sha256.Sum256(plaintext)
	if !bytes.Equal(checksum[:], digestBytes) {
		return result, errNoteTampered
	}

	if err := json.Unmarshal(plaintext, &result); err != nil {
		return result, err
	}

	return result, nil
}

func generateNoteID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (m *Manager) SaveNote(input Note, expectedVersion int) (Note, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	noteID := strings.TrimSpace(input.ID)
	if noteID == "" {
		var err error
		noteID, err = generateNoteID()
		if err != nil {
			return Note{}, fmt.Errorf("generate note identifier: %w", err)
		}
	}

	existing, exists := m.notes[noteID]
	if exists {
		if expectedVersion > 0 && existing.Version != expectedVersion {
			return Note{}, errNoteConflict
		}
	} else if expectedVersion > 0 {
		return Note{}, errNoteConflict
	}

	content := noteContent{Title: strings.TrimSpace(input.Title), Body: input.Body}
	ciphertext, nonce, digest, err := encryptNote(m.keyFor(input.Shared), content)
	if err != nil {
		return Note{}, fmt.Errorf("encrypt note: %w", err)
	}

	version := 1
	if exists {
		version = existing.Version + 1
	}

	stored := storedNote{
		ID:         noteID,
		Shared:     input.Shared,
		Version:    version,
		UpdatedAt:  time.Now().UTC(),
		Ciphertext: ciphertext,
		Nonce:      nonce,
		Digest:     digest,
	}

	m.notes[noteID] = stored
	if stored.Shared {
		m.dirty = true
	}

	if err := m.persistLocked(); err != nil {
		return Note{}, err
	}

	return Note{
		ID:        stored.ID,
		Title:     content.Title,
		Body:      content.Body,
		Shared:    stored.Shared,
		Version:   stored.Version,
		UpdatedAt: stored.UpdatedAt,
	}, nil
}

func (m *Manager) ListNotes() ([]Note, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	notes := make([]Note, 0, len(m.notes))
	ids := make([]string, 0, len(m.notes))
	for id := range m.notes {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	migrated := false
	for _, id := range ids {
		stored := m.notes[id]
		content, migratedNote, err := m.decryptStoredLocked(stored)
		if err != nil {
			continue
		}
		if migratedNote {
			migrated = true
		}
		notes = append(notes, Note{
			ID:        stored.ID,
			Title:     content.Title,
			Body:      content.Body,
			Shared:    stored.Shared,
			Version:   stored.Version,
			UpdatedAt: stored.UpdatedAt,
		})
	}

	if migrated {
		if err := m.persistLocked(); err != nil {
			return nil, err
		}
	}

	return notes, nil
}

func (m *Manager) SyncShared(ctx context.Context, client *http.Client, baseURL, agentID, agentKey, userAgent string) error {
	if client == nil || strings.TrimSpace(baseURL) == "" || strings.TrimSpace(agentID) == "" || strings.TrimSpace(agentKey) == "" {
		return nil
	}

	m.mu.RLock()
	sharedNotes := make([]storedNote, 0)
	for _, note := range m.notes {
		if note.Shared {
			sharedNotes = append(sharedNotes, note)
		}
	}
	dirty := m.dirty
	lastSync := m.lastSync
	interval := m.syncInterval
	m.mu.RUnlock()

	if len(sharedNotes) == 0 && !dirty {
		if time.Since(lastSync) < interval {
			return nil
		}
	}

	sort.Slice(sharedNotes, func(i, j int) bool {
		return sharedNotes[i].ID < sharedNotes[j].ID
	})

	payload := noteSyncRequest{Notes: make([]noteEnvelope, len(sharedNotes))}
	for i, note := range sharedNotes {
		payload.Notes[i] = noteEnvelope{
			ID:         note.ID,
			Visibility: "shared",
			UpdatedAt:  note.UpdatedAt.Format(time.RFC3339Nano),
			Version:    note.Version,
			Ciphertext: note.Ciphertext,
			Nonce:      note.Nonce,
			Digest:     note.Digest,
		}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/api/agents/%s/notes", strings.TrimRight(baseURL, "/"), url.PathEscape(agentID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if ua := strings.TrimSpace(userAgent); ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", agentKey))

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return protocol.ErrUnauthorized
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return fmt.Errorf("note sync failed: %s", message)
	}

	var response noteSyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	changed := false
	for _, envelope := range response.Notes {
		if strings.ToLower(envelope.Visibility) != "shared" {
			continue
		}
		updatedAt, err := time.Parse(time.RFC3339Nano, envelope.UpdatedAt)
		if err != nil {
			continue
		}
		stored := storedNote{
			ID:         envelope.ID,
			Shared:     true,
			Version:    envelope.Version,
			UpdatedAt:  updatedAt.UTC(),
			Ciphertext: envelope.Ciphertext,
			Nonce:      envelope.Nonce,
			Digest:     envelope.Digest,
		}

		existing, exists := m.notes[stored.ID]
		if !exists || stored.UpdatedAt.After(existing.UpdatedAt) || stored.Version > existing.Version {
			m.notes[stored.ID] = stored
			changed = true
		}
	}

	m.lastSync = time.Now()
	if len(response.Notes) > 0 || dirty {
		m.dirty = false
	}

	if changed {
		if err := m.persistLocked(); err != nil {
			return err
		}
	}

	return nil
}

func DefaultPath(baseDir string) (string, error) {
	trimmed := strings.TrimSpace(baseDir)
	if trimmed == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		trimmed = filepath.Join(home, ".config", "tenvy")
	}
	return filepath.Join(trimmed, "notes.json"), nil
}
