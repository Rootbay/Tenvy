package plugins

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	manifest "github.com/rootbay/tenvy-client/shared/pluginmanifest"
)

const manifestFileName = "manifest.json"

// Manager inspects local plugin artifacts and produces telemetry snapshots that
// can be forwarded to the controller.
type Manager struct {
	root   string
	logger *log.Logger
}

// NewManager creates a plugin manager rooted at the provided directory.
func NewManager(root string, logger *log.Logger) (*Manager, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("plugin root directory not provided")
	}
	if logger == nil {
		logger = log.New(os.Stderr, "", log.LstdFlags)
	}
	return &Manager{root: root, logger: logger}, nil
}

// Snapshot walks the plugin root and returns the current installation
// telemetry. Errors are logged and omitted from the resulting payload.
func (m *Manager) Snapshot() *manifest.SyncPayload {
	entries, err := os.ReadDir(m.root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		m.logger.Printf("plugin scan failed: %v", err)
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	payload := manifest.SyncPayload{Installations: make([]manifest.InstallationTelemetry, 0, len(entries))}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pluginDir := filepath.Join(m.root, entry.Name())
		manifestPath := filepath.Join(pluginDir, manifestFileName)

		manifestData, err := os.ReadFile(manifestPath)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				m.logger.Printf("plugin %s missing manifest: %v", entry.Name(), err)
			}
			continue
		}

		var mf manifest.Manifest
		if err := json.Unmarshal(manifestData, &mf); err != nil {
			m.logger.Printf("plugin %s manifest parse failed: %v", entry.Name(), err)
			continue
		}
		if err := mf.Validate(); err != nil {
			m.logger.Printf("plugin %s manifest invalid: %v", mf.ID, err)
			continue
		}

		installation := manifest.InstallationTelemetry{
			PluginID: mf.ID,
			Version:  mf.Version,
			Status:   manifest.InstallPending,
		}

		artifactRel := filepath.Clean(mf.Package.Artifact)
		if strings.HasPrefix(artifactRel, "..") {
			installation.Status = manifest.InstallFailed
			installation.Error = "artifact path escapes plugin directory"
			installation.LastCheckedAt = &now
			payload.Installations = append(payload.Installations, installation)
			continue
		}

		artifactPath := filepath.Join(pluginDir, artifactRel)
		info, statErr := os.Stat(artifactPath)
		switch {
		case statErr == nil && !info.IsDir():
			hash, hashErr := fileHash(artifactPath)
			if hashErr != nil {
				installation.Status = manifest.InstallFailed
				installation.Error = fmt.Sprintf("hash: %v", hashErr)
			} else {
				installation.Hash = hash
				if mf.Package.Hash != "" && !strings.EqualFold(mf.Package.Hash, hash) {
					installation.Status = manifest.InstallFailed
					installation.Error = "hash mismatch"
				} else {
					installation.Status = manifest.InstallInstalled
					ts := info.ModTime().UTC().Format(time.RFC3339Nano)
					installation.LastDeployedAt = &ts
				}
			}
		case errors.Is(statErr, fs.ErrNotExist):
			installation.Status = manifest.InstallPending
			installation.Error = "artifact missing"
		case statErr != nil:
			installation.Status = manifest.InstallFailed
			installation.Error = statErr.Error()
		default:
			installation.Status = manifest.InstallFailed
			installation.Error = "artifact is a directory"
		}

		installation.LastCheckedAt = &now
		payload.Installations = append(payload.Installations, installation)
	}

	if len(payload.Installations) == 0 {
		return nil
	}

	return &payload
}

func fileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	sum := hasher.Sum(nil)
	return hex.EncodeToString(sum), nil
}
