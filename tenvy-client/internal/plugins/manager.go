package plugins

import (
	"crypto/ed25519"
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
	"sync"
	"time"

	manifest "github.com/rootbay/tenvy-client/shared/pluginmanifest"
)

const manifestFileName = "manifest.json"

// Manager inspects local plugin artifacts and produces telemetry snapshots that
// can be forwarded to the controller.
type Manager struct {
	root      string
	logger    *log.Logger
	verifyMu  sync.RWMutex
	verifyOpt manifest.VerifyOptions
}

// NewManager creates a plugin manager rooted at the provided directory.
func NewManager(root string, logger *log.Logger, verify manifest.VerifyOptions) (*Manager, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("plugin root directory not provided")
	}
	if logger == nil {
		logger = log.New(os.Stderr, "", log.LstdFlags)
	}
	manager := &Manager{root: root, logger: logger}
	manager.UpdateVerification(verify)
	return manager, nil
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

		verificationResult, verifyErr := manifest.VerifySignature(mf, m.verificationOptions())
		if verifyErr != nil {
			installation.Status = manifest.InstallBlocked
			installation.Error = fmt.Sprintf("signature: %s", signatureErrorMessage(verifyErr))
			installation.LastCheckedAt = &now
			payload.Installations = append(payload.Installations, installation)
			continue
		}

		if verificationResult == nil || !verificationResult.Trusted {
			installation.Status = manifest.InstallBlocked
			installation.Error = fmt.Sprintf("signature: %s", signatureUntrustedReason(mf, verificationResult))
			installation.LastCheckedAt = &now
			payload.Installations = append(payload.Installations, installation)
			continue
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

// UpdateVerification swaps the verification options used by the manager.
func (m *Manager) UpdateVerification(opts manifest.VerifyOptions) {
	m.verifyMu.Lock()
	m.verifyOpt = cloneVerifyOptions(opts)
	m.verifyMu.Unlock()
}

func (m *Manager) verificationOptions() manifest.VerifyOptions {
	m.verifyMu.RLock()
	defer m.verifyMu.RUnlock()
	return cloneVerifyOptions(m.verifyOpt)
}

func cloneVerifyOptions(opts manifest.VerifyOptions) manifest.VerifyOptions {
	clone := opts
	if len(opts.SHA256AllowList) > 0 {
		clone.SHA256AllowList = append([]string(nil), opts.SHA256AllowList...)
	}
	if len(opts.Ed25519PublicKeys) > 0 {
		clone.Ed25519PublicKeys = make(map[string]ed25519.PublicKey, len(opts.Ed25519PublicKeys))
		for keyID, key := range opts.Ed25519PublicKeys {
			clone.Ed25519PublicKeys[keyID] = append(ed25519.PublicKey(nil), key...)
		}
	}
	return clone
}

func signatureErrorMessage(err error) string {
	switch {
	case errors.Is(err, manifest.ErrUnsignedPlugin):
		return "unsigned plugin"
	case errors.Is(err, manifest.ErrSignatureMismatch):
		return "hash mismatch"
	case errors.Is(err, manifest.ErrHashNotAllowed):
		return "hash not allowed"
	case errors.Is(err, manifest.ErrUntrustedSigner):
		return "untrusted signer"
	case errors.Is(err, manifest.ErrInvalidSignature):
		return "invalid signature"
	case errors.Is(err, manifest.ErrSignatureExpired):
		return "signature expired"
	case errors.Is(err, manifest.ErrSignatureNotYetValid):
		return "signature timestamp in future"
	default:
		return err.Error()
	}
}

func signatureUntrustedReason(mf manifest.Manifest, result *manifest.VerificationResult) string {
	if result == nil {
		return "signature not trusted"
	}

	switch result.SignatureType {
	case manifest.SignatureNone:
		return "unsigned plugin"
	case manifest.SignatureSHA256:
		return "hash not trusted"
	case manifest.SignatureEd25519:
		if strings.TrimSpace(result.Signer) != "" {
			return fmt.Sprintf("untrusted signer %s", result.Signer)
		}
		if strings.TrimSpace(mf.Distribution.Signature.PublicKey) != "" {
			return fmt.Sprintf("untrusted key %s", mf.Distribution.Signature.PublicKey)
		}
		return "untrusted signer"
	default:
		return "untrusted signature"
	}
}
