package agent

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	remotedesktop "github.com/rootbay/tenvy-client/internal/modules/control/remotedesktop"
	"github.com/rootbay/tenvy-client/internal/plugins"
	"github.com/rootbay/tenvy-client/internal/protocol"
	manifest "github.com/rootbay/tenvy-client/shared/pluginmanifest"
)

func TestRemoteDesktopModuleNegotiationWithManagedEngine(t *testing.T) {
	const (
		agentID       = "agent-1"
		pluginVersion = "1.2.3"
		sessionID     = "session-123"
	)

	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "engine.log")
	t.Setenv("REMOTE_DESKTOP_ENGINE_LOG", logPath)

	artifactData := buildEngineArtifact(t)
	artifactHash := sha256.Sum256(artifactData)
	manifestJSON, err := json.Marshal(manifest.Manifest{
		ID:            plugins.RemoteDesktopEnginePluginID,
		Name:          "Remote Desktop Engine",
		Version:       pluginVersion,
		Entry:         "remote-desktop-engine/engine.sh",
		RepositoryURL: "https://github.com/rootbay/remote-desktop-engine",
		License:       manifest.LicenseInfo{SPDXID: "MIT"},
		Requirements: manifest.Requirements{
			MinAgentVersion: "0.1.0",
			RequiredModules: []string{"remote-desktop"},
		},
		Distribution: manifest.Distribution{
			DefaultMode: "automatic",
			AutoUpdate:  true,
			Signature:   manifest.Signature{Type: manifest.SignatureNone},
		},
		Package: manifest.PackageDescriptor{
			Artifact: "remote-desktop-engine/engine.zip",
			Hash:     fmt.Sprintf("%x", artifactHash[:]),
		},
	})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	handshakeCh := make(chan remotedesktop.RemoteDesktopSessionNegotiationRequest, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/artifact"):
			w.Header().Set("Content-Type", "application/octet-stream")
			if _, err := w.Write(artifactData); err != nil {
				t.Fatalf("write artifact: %v", err)
			}
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/plugins/"+plugins.RemoteDesktopEnginePluginID):
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write(manifestJSON); err != nil {
				t.Fatalf("write manifest: %v", err)
			}
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/remote-desktop/transport"):
			defer r.Body.Close()
			var req remotedesktop.RemoteDesktopSessionNegotiationRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid negotiation payload", http.StatusBadRequest)
				return
			}
			select {
			case handshakeCh <- req:
			default:
			}
			w.Header().Set("Content-Type", "application/json")
			resp := remotedesktop.RemoteDesktopSessionNegotiationResponse{
				Accepted:              true,
				Transport:             remotedesktop.RemoteTransportHTTP,
				Codec:                 remotedesktop.RemoteEncoderJPEG,
				RequiredPluginVersion: pluginVersion,
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("write negotiation response: %v", err)
			}
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/remote-desktop/frames"):
			io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	manager, err := plugins.NewManager(t.TempDir(), log.New(io.Discard, "", 0), manifest.VerifyOptions{AllowUnsigned: true})
	if err != nil {
		t.Fatalf("new plugin manager: %v", err)
	}

	runtime := ModuleRuntime{
		AgentID:    agentID,
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		Logger:     log.New(io.Discard, "", 0),
		UserAgent:  "integration-test",
		Plugins:    manager,
	}

	module := newRemoteDesktopModule(nil)
	if err := module.Init(context.Background(), runtime); err != nil {
		t.Fatalf("module init: %v", err)
	}

	module.mu.Lock()
	configuredVersion := module.engineConfig.PluginVersion
	module.mu.Unlock()
	if strings.TrimSpace(configuredVersion) != pluginVersion {
		t.Fatalf("expected module to configure plugin version %s, got %q", pluginVersion, configuredVersion)
	}

	startPayload := remotedesktop.RemoteDesktopCommandPayload{Action: "start", SessionID: sessionID}
	startRaw, err := json.Marshal(startPayload)
	if err != nil {
		t.Fatalf("marshal start payload: %v", err)
	}

	startCmd := protocol.Command{
		ID:        "cmd-start",
		Name:      "remote-desktop",
		Payload:   startRaw,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}

	result := module.Handle(context.Background(), startCmd)
	if !result.Success {
		t.Fatalf("start command failed: %s", result.Error)
	}

	select {
	case req := <-handshakeCh:
		if strings.TrimSpace(req.PluginVersion) != pluginVersion {
			t.Fatalf("expected plugin version %s, got %q", pluginVersion, req.PluginVersion)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for negotiation request")
	}

	waitForEngineLog(t, logPath, sessionID)

	stopPayload := remotedesktop.RemoteDesktopCommandPayload{Action: "stop", SessionID: sessionID}
	stopRaw, err := json.Marshal(stopPayload)
	if err != nil {
		t.Fatalf("marshal stop payload: %v", err)
	}
	stopCmd := protocol.Command{
		ID:        "cmd-stop",
		Name:      "remote-desktop",
		Payload:   stopRaw,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	stopResult := module.Handle(context.Background(), stopCmd)
	if !stopResult.Success {
		t.Fatalf("stop command failed: %s", stopResult.Error)
	}

	module.Shutdown(context.Background())

	snapshot := manager.Snapshot()
	if snapshot == nil || len(snapshot.Installations) != 1 {
		t.Fatalf("expected plugin snapshot entry, got %#v", snapshot)
	}
	install := snapshot.Installations[0]
	if install.PluginID != plugins.RemoteDesktopEnginePluginID {
		t.Fatalf("unexpected plugin id %s", install.PluginID)
	}
	if install.Version != pluginVersion {
		t.Fatalf("expected version %s, got %s", pluginVersion, install.Version)
	}
	if install.Status != manifest.InstallInstalled {
		t.Fatalf("expected installed status, got %s", install.Status)
	}
}

func buildEngineArtifact(t *testing.T) []byte {
	t.Helper()
	script := "#!/bin/sh\n" +
		"if [ -n \"$REMOTE_DESKTOP_ENGINE_LOG\" ]; then echo \"started:$TENVY_REMOTE_DESKTOP_SESSION_ID\" >> \"$REMOTE_DESKTOP_ENGINE_LOG\"; fi\n" +
		"trap exit TERM INT\n" +
		"while true; do sleep 1; done\n"

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	header := &zip.FileHeader{Name: "remote-desktop-engine/engine.sh", Method: zip.Deflate}
	header.SetMode(0o755)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatalf("create zip header: %v", err)
	}
	if _, err := entry.Write([]byte(script)); err != nil {
		t.Fatalf("write script: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buf.Bytes()
}

func waitForEngineLog(t *testing.T, path, sessionID string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), sessionID) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	data, _ := os.ReadFile(path)
	t.Fatalf("engine log missing entry for session %s (contents: %q)", sessionID, string(data))
}
