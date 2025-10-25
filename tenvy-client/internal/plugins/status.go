package plugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	manifest "github.com/rootbay/tenvy-client/shared/pluginmanifest"
)

const statusFileName = ".status.json"

type installationStatusRecord struct {
	ID        string                       `json:"pluginId,omitempty"`
	Version   string                       `json:"version,omitempty"`
	Status    manifest.PluginInstallStatus `json:"status,omitempty"`
	Error     string                       `json:"error,omitempty"`
	Timestamp string                       `json:"timestamp,omitempty"`
}

func normalizeInstallStatus(status manifest.PluginInstallStatus) manifest.PluginInstallStatus {
	switch strings.ToLower(strings.TrimSpace(string(status))) {
	case string(manifest.InstallInstalled):
		return manifest.InstallInstalled
	case string(manifest.InstallBlocked):
		return manifest.InstallBlocked
	case string(manifest.InstallDisabled):
		return manifest.InstallDisabled
	case string(manifest.InstallError), "failed", "pending", "installing":
		return manifest.InstallError
	default:
		return manifest.InstallError
	}
}

func (r *installationStatusRecord) PluginID(defaultID string) string {
	if r == nil {
		return strings.TrimSpace(defaultID)
	}
	if id := strings.TrimSpace(r.ID); id != "" {
		return id
	}
	return strings.TrimSpace(defaultID)
}

func loadInstallationStatus(dir string) *installationStatusRecord {
	path := filepath.Join(dir, statusFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var record installationStatusRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil
	}
	record.Status = normalizeInstallStatus(record.Status)
	return &record
}

func writeInstallationStatus(dir string, record installationStatusRecord) error {
	if strings.TrimSpace(dir) == "" {
		return errors.New("plugin status directory not provided")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensure plugin status directory: %w", err)
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal installation status: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "status-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary status file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write status data: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close status file: %w", err)
	}
	target := filepath.Join(dir, statusFileName)
	if err := os.Rename(tmpPath, target); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("persist status file: %w", err)
	}
	return nil
}

func removeInstallationStatus(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return errors.New("plugin status directory not provided")
	}
	path := filepath.Join(dir, statusFileName)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove status file: %w", err)
	}
	return nil
}

func (m *Manager) recordInstallStatusLocked(pluginID, version string, status manifest.PluginInstallStatus, message string) error {
	if m == nil {
		return errors.New("plugins manager is nil")
	}
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return errors.New("plugin id not provided")
	}
	dir := filepath.Join(m.root, pluginID)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := installationStatusRecord{
		ID:        pluginID,
		Version:   strings.TrimSpace(version),
		Status:    normalizeInstallStatus(status),
		Error:     strings.TrimSpace(message),
		Timestamp: now,
	}
	return writeInstallationStatus(dir, record)
}

func (m *Manager) clearInstallStatusLocked(pluginID string) error {
	if m == nil {
		return nil
	}
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return nil
	}
	dir := filepath.Join(m.root, pluginID)
	return removeInstallationStatus(dir)
}

// RecordInstallStatus persists a plugin installation status update to disk so that
// telemetry snapshots can surface the latest failure or health state to the
// controller.
func RecordInstallStatus(m *Manager, pluginID, version string, status manifest.PluginInstallStatus, message string) error {
	if m == nil {
		return errors.New("plugins manager not initialized")
	}
	m.stageMu.Lock()
	defer m.stageMu.Unlock()
	return m.recordInstallStatusLocked(pluginID, version, status, message)
}

// ClearInstallStatus removes any persisted status overrides for the provided plugin.
func ClearInstallStatus(m *Manager, pluginID string) error {
	if m == nil {
		return nil
	}
	m.stageMu.Lock()
	defer m.stageMu.Unlock()
	return m.clearInstallStatusLocked(pluginID)
}
