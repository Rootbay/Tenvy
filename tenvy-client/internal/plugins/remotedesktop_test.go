package plugins_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	plugins "github.com/rootbay/tenvy-client/internal/plugins"
	manifest "github.com/rootbay/tenvy-client/shared/pluginmanifest"
)

func TestStageRemoteDesktopEngineSuccess(t *testing.T) {
	t.Parallel()

	artifactData := buildRemoteDesktopArtifact(t, map[string][]byte{
		"remote-desktop-engine/remote-desktop-engine": []byte("engine payload"),
	})
	hash := sha256.Sum256(artifactData)
	hashHex := fmt.Sprintf("%x", hash[:])

	manifestJSON := fmt.Sprintf(`{
                "id": "remote-desktop-engine",
                "name": "Remote Desktop Engine",
                "version": "9.9.9",
                "entry": "remote-desktop-engine/remote-desktop-engine",
                "repositoryUrl": "https://github.com/rootbay/tenvy-client",
                "license": { "spdxId": "MIT" },
                "requirements": {},
                "distribution": {"defaultMode": "automatic", "autoUpdate": true, "signature": {"type": "sha256", "hash": "%[1]s", "signature": "%[1]s"}},
                "package": {"artifact": "remote-desktop-engine/remote-desktop-engine.zip", "hash": "%s"}
        }`, hashHex, hashHex)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/artifact") {
			w.Header().Set("Content-Type", "application/octet-stream")
			if _, err := w.Write(artifactData); err != nil {
				t.Fatalf("write artifact: %v", err)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, manifestJSON); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	opts := manifest.VerifyOptions{SHA256AllowList: []string{hashHex}}
	manager, err := plugins.NewManager(root, log.New(io.Discard, "", 0), opts)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	ctx := context.Background()
	result, err := plugins.StageRemoteDesktopEngine(ctx, manager, server.Client(), server.URL, "agent-1", "", "stage-test", manifest.RuntimeFacts{})
	if err != nil {
		t.Fatalf("stage engine: %v", err)
	}
	if !result.Updated {
		t.Fatalf("expected install to be marked updated")
	}
	if strings.TrimSpace(result.EntryPath) == "" {
		t.Fatalf("expected entry path, got empty string")
	}

	entryPayload, err := os.ReadFile(result.EntryPath)
	if err != nil {
		t.Fatalf("read entry payload: %v", err)
	}
	if string(entryPayload) != "engine payload" {
		t.Fatalf("unexpected entry payload %q", string(entryPayload))
	}

	artifactPath := filepath.Join(manager.Root(), plugins.RemoteDesktopEnginePluginID, "remote-desktop-engine", "remote-desktop-engine.zip")
	if _, err := os.Stat(artifactPath); err != nil {
		t.Fatalf("expected artifact persisted: %v", err)
	}

	snapshot := manager.Snapshot()
	if snapshot == nil || len(snapshot.Installations) != 1 {
		t.Fatalf("expected snapshot entry, got %#v", snapshot)
	}
	if snapshot.Installations[0].Status != manifest.InstallInstalled {
		t.Fatalf("expected installed status, got %s", snapshot.Installations[0].Status)
	}

	// Subsequent staging should be a no-op.
	result2, err := plugins.StageRemoteDesktopEngine(ctx, manager, server.Client(), server.URL, "agent-1", "", "stage-test", manifest.RuntimeFacts{})
	if err != nil {
		t.Fatalf("restage engine: %v", err)
	}
	if result2.Updated {
		t.Fatalf("expected restage to be no-op")
	}
}

func TestStageRemoteDesktopEngineRecordsFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	root := t.TempDir()
	manager, err := plugins.NewManager(root, log.New(io.Discard, "", 0), manifest.VerifyOptions{})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	_, err = plugins.StageRemoteDesktopEngine(context.Background(), manager, server.Client(), server.URL, "agent-1", "", "stage-test", manifest.RuntimeFacts{})
	if err == nil {
		t.Fatal("expected staging to fail")
	}

	snapshot := manager.Snapshot()
	if snapshot == nil || len(snapshot.Installations) != 1 {
		t.Fatalf("expected failure snapshot entry, got %#v", snapshot)
	}
	install := snapshot.Installations[0]
	if install.Status != manifest.InstallFailed {
		t.Fatalf("expected failure status, got %s", install.Status)
	}
	if !strings.Contains(install.Error, "boom") {
		t.Fatalf("expected controller error message, got %q", install.Error)
	}
}

func TestStageRemoteDesktopEngineBlocksIncompatiblePlatform(t *testing.T) {
	t.Parallel()

	hashHex := strings.Repeat("ab", 32)
	manifestJSON := fmt.Sprintf(`{
                "id": "remote-desktop-engine",
                "name": "Remote Desktop Engine",
                "version": "1.0.0",
                "entry": "remote-desktop-engine/remote-desktop-engine",
                "repositoryUrl": "https://github.com/rootbay/tenvy-client",
                "license": {"spdxId": "MIT"},
                "requirements": {"platforms": ["windows"]},
                "distribution": {"defaultMode": "manual", "autoUpdate": false, "signature": {"type": "sha256", "hash": "%[1]s", "signature": "%[1]s"}},
                "package": {"artifact": "remote-desktop-engine/remote-desktop-engine.zip", "hash": "%[1]s"}
        }`, hashHex)

	var artifactRequested atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/artifact") {
			artifactRequested.Store(true)
			t.Fatalf("artifact download should not be attempted")
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, manifestJSON); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	opts := manifest.VerifyOptions{SHA256AllowList: []string{hashHex}}
	manager, err := plugins.NewManager(root, log.New(io.Discard, "", 0), opts)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	facts := manifest.RuntimeFacts{
		Platform:       "linux",
		Architecture:   "x86_64",
		AgentVersion:   "1.0.0",
		EnabledModules: []string{"remote-desktop"},
	}

	_, err = plugins.StageRemoteDesktopEngine(context.Background(), manager, server.Client(), server.URL, "agent-1", "", "stage-test", facts)
	if err == nil {
		t.Fatal("expected staging to be blocked")
	}
	if !strings.Contains(err.Error(), "platform") {
		t.Fatalf("expected platform error, got %v", err)
	}
	if artifactRequested.Load() {
		t.Fatal("expected no artifact download attempts")
	}

	snapshot := manager.Snapshot()
	if snapshot == nil || len(snapshot.Installations) != 1 {
		t.Fatalf("expected snapshot entry, got %#v", snapshot)
	}
	install := snapshot.Installations[0]
	if install.Status != manifest.InstallBlocked {
		t.Fatalf("expected blocked status, got %s", install.Status)
	}
	if !strings.Contains(install.Error, "platform") {
		t.Fatalf("expected platform message, got %q", install.Error)
	}
}

func TestStageRemoteDesktopEngineBlocksIncompatibleArchitecture(t *testing.T) {
	t.Parallel()

	hashHex := strings.Repeat("cd", 32)
	manifestJSON := fmt.Sprintf(`{
                "id": "remote-desktop-engine",
                "name": "Remote Desktop Engine",
                "version": "1.0.0",
                "entry": "remote-desktop-engine/remote-desktop-engine",
                "repositoryUrl": "https://github.com/rootbay/tenvy-client",
                "license": {"spdxId": "MIT"},
                "requirements": {"architectures": ["arm64"]},
                "distribution": {"defaultMode": "manual", "autoUpdate": false, "signature": {"type": "sha256", "hash": "%[1]s", "signature": "%[1]s"}},
                "package": {"artifact": "remote-desktop-engine/remote-desktop-engine.zip", "hash": "%[1]s"}
        }`, hashHex)

	var artifactRequested atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/artifact") {
			artifactRequested.Store(true)
			t.Fatalf("artifact download should not be attempted")
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, manifestJSON); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	opts := manifest.VerifyOptions{SHA256AllowList: []string{hashHex}}
	manager, err := plugins.NewManager(root, log.New(io.Discard, "", 0), opts)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	facts := manifest.RuntimeFacts{
		Platform:       "linux",
		Architecture:   "x86_64",
		AgentVersion:   "1.0.0",
		EnabledModules: []string{"remote-desktop"},
	}

	_, err = plugins.StageRemoteDesktopEngine(context.Background(), manager, server.Client(), server.URL, "agent-1", "", "stage-test", facts)
	if err == nil {
		t.Fatal("expected staging to be blocked")
	}
	if !strings.Contains(err.Error(), "architecture") {
		t.Fatalf("expected architecture error, got %v", err)
	}
	if artifactRequested.Load() {
		t.Fatal("expected no artifact download attempts")
	}

	snapshot := manager.Snapshot()
	if snapshot == nil || len(snapshot.Installations) != 1 {
		t.Fatalf("expected snapshot entry, got %#v", snapshot)
	}
	install := snapshot.Installations[0]
	if install.Status != manifest.InstallBlocked {
		t.Fatalf("expected blocked status, got %s", install.Status)
	}
	if !strings.Contains(install.Error, "architecture") {
		t.Fatalf("expected architecture message, got %q", install.Error)
	}
}

func TestStageRemoteDesktopEngineBlocksIncompatibleAgentVersion(t *testing.T) {
	t.Parallel()

	hashHex := strings.Repeat("ef", 32)
	manifestJSON := fmt.Sprintf(`{
                "id": "remote-desktop-engine",
                "name": "Remote Desktop Engine",
                "version": "1.0.0",
                "entry": "remote-desktop-engine/remote-desktop-engine",
                "repositoryUrl": "https://github.com/rootbay/tenvy-client",
                "license": {"spdxId": "MIT"},
                "requirements": {"minAgentVersion": "5.0.0"},
                "distribution": {"defaultMode": "manual", "autoUpdate": false, "signature": {"type": "sha256", "hash": "%[1]s", "signature": "%[1]s"}},
                "package": {"artifact": "remote-desktop-engine/remote-desktop-engine.zip", "hash": "%[1]s"}
        }`, hashHex)

	var artifactRequested atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/artifact") {
			artifactRequested.Store(true)
			t.Fatalf("artifact download should not be attempted")
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, manifestJSON); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	opts := manifest.VerifyOptions{SHA256AllowList: []string{hashHex}}
	manager, err := plugins.NewManager(root, log.New(io.Discard, "", 0), opts)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	facts := manifest.RuntimeFacts{
		Platform:       "linux",
		Architecture:   "x86_64",
		AgentVersion:   "1.0.0",
		EnabledModules: []string{"remote-desktop"},
	}

	_, err = plugins.StageRemoteDesktopEngine(context.Background(), manager, server.Client(), server.URL, "agent-1", "", "stage-test", facts)
	if err == nil {
		t.Fatal("expected staging to be blocked")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Fatalf("expected version error, got %v", err)
	}
	if artifactRequested.Load() {
		t.Fatal("expected no artifact download attempts")
	}

	snapshot := manager.Snapshot()
	if snapshot == nil || len(snapshot.Installations) != 1 {
		t.Fatalf("expected snapshot entry, got %#v", snapshot)
	}
	install := snapshot.Installations[0]
	if install.Status != manifest.InstallBlocked {
		t.Fatalf("expected blocked status, got %s", install.Status)
	}
	if !strings.Contains(install.Error, "version") {
		t.Fatalf("expected version message, got %q", install.Error)
	}
}

func buildRemoteDesktopArtifact(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, payload := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := entry.Write(payload); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buf.Bytes()
}
