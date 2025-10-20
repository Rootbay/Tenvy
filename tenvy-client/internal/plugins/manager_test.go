package plugins_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	plugins "github.com/rootbay/tenvy-client/internal/plugins"
	manifest "github.com/rootbay/tenvy-client/shared/pluginmanifest"
)

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func TestSnapshotBlocksUnsignedPlugin(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pluginDir := filepath.Join(root, "unsigned")
	artifactPath := filepath.Join(pluginDir, "plugin.dll")

	manifestPath := filepath.Join(pluginDir, "manifest.json")
	writeFile(t, manifestPath, []byte(`{
                "id": "unsigned",
                "name": "Unsigned",
                "version": "1.0.0",
                "entry": "plugin.dll",
                "repositoryUrl": "https://github.com/rootbay/unsigned",
                "license": { "spdxId": "MIT" },
                "requirements": {},
                "distribution": {"defaultMode": "manual", "autoUpdate": false, "signature": {"type": "none"}},
                "package": {"artifact": "plugin.dll"}
        }`))
	writeFile(t, artifactPath, []byte("payload"))

	opts := manifest.VerifyOptions{AllowUnsigned: false}
	manager, err := plugins.NewManager(root, log.New(io.Discard, "", 0), opts)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	snapshot := manager.Snapshot()
	if snapshot == nil || len(snapshot.Installations) != 1 {
		t.Fatalf("expected one installation, got %#v", snapshot)
	}
	install := snapshot.Installations[0]
	if install.Status != manifest.InstallBlocked {
		t.Fatalf("expected status blocked, got %s", install.Status)
	}
	if install.Error == "" || !strings.Contains(install.Error, "unsigned") {
		t.Fatalf("expected unsigned error, got %q", install.Error)
	}
}

func TestSnapshotAllowsTrustedSignature(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pluginDir := filepath.Join(root, "trusted")
	artifactPath := filepath.Join(pluginDir, "plugin.bin")

	payload := make([]byte, 128)
	if _, err := rand.Read(payload); err != nil {
		t.Fatalf("rand: %v", err)
	}
	writeFile(t, artifactPath, payload)

	hash := sha256SumHex(payload)

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signature := ed25519.Sign(priv, []byte(hash))

	manifestJSON := fmt.Sprintf(`{
                "id": "trusted",
                "name": "Trusted",
                "version": "1.0.0",
                "entry": "plugin.bin",
                "repositoryUrl": "https://github.com/rootbay/trusted",
                "license": { "spdxId": "MIT" },
                "requirements": {},
                "distribution": {
                        "defaultMode": "manual",
                        "autoUpdate": false,
                        "signature": {
                                "type": "ed25519",
                                "hash": "%s",
                                "publicKey": "primary",
                                "signature": "%s"
                        }
                },
                "package": {"artifact": "plugin.bin", "hash": "%s"}
        }`, hash, hex.EncodeToString(signature), hash)
	writeFile(t, filepath.Join(pluginDir, "manifest.json"), []byte(manifestJSON))

	opts := manifest.VerifyOptions{
		Ed25519PublicKeys: map[string]ed25519.PublicKey{"primary": pub},
	}
	manager, err := plugins.NewManager(root, log.New(io.Discard, "", 0), opts)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	snapshot := manager.Snapshot()
	if snapshot == nil || len(snapshot.Installations) != 1 {
		t.Fatalf("expected one installation, got %#v", snapshot)
	}
	install := snapshot.Installations[0]
	if install.Status != manifest.InstallInstalled {
		t.Fatalf("expected status installed, got %s", install.Status)
	}
	if !strings.EqualFold(install.Hash, hash) {
		t.Fatalf("expected hash %s, got %s", hash, install.Hash)
	}
}

func TestSnapshotBlocksInvalidSignature(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pluginDir := filepath.Join(root, "invalid")
	artifactPath := filepath.Join(pluginDir, "plugin.bin")

	payload := []byte("data")
	writeFile(t, artifactPath, payload)
	hash := sha256SumHex(payload)

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	manifestJSON := fmt.Sprintf(`{
                "id": "invalid",
                "name": "Invalid",
                "version": "1.0.0",
                "entry": "plugin.bin",
                "repositoryUrl": "https://github.com/rootbay/invalid",
                "license": { "spdxId": "MIT" },
                "requirements": {},
                "distribution": {
                        "defaultMode": "manual",
                        "autoUpdate": false,
                        "signature": {
                                "type": "ed25519",
                                "hash": "%s",
                                "publicKey": "primary",
                                "signature": "%s"
                        }
                },
                "package": {"artifact": "plugin.bin", "hash": "%s"}
        }`, hash, strings.Repeat("00", ed25519.SignatureSize), hash)
	writeFile(t, filepath.Join(pluginDir, "manifest.json"), []byte(manifestJSON))

	opts := manifest.VerifyOptions{
		Ed25519PublicKeys: map[string]ed25519.PublicKey{"primary": pub},
	}
	manager, err := plugins.NewManager(root, log.New(io.Discard, "", 0), opts)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	snapshot := manager.Snapshot()
	if snapshot == nil || len(snapshot.Installations) != 1 {
		t.Fatalf("expected one installation, got %#v", snapshot)
	}
	install := snapshot.Installations[0]
	if install.Status != manifest.InstallBlocked {
		t.Fatalf("expected blocked status, got %s", install.Status)
	}
	if install.Error == "" || !strings.Contains(install.Error, "invalid") {
		t.Fatalf("expected invalid signature error, got %q", install.Error)
	}
}

func sha256SumHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
