package agent

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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

	artifactData := buildEngineArtifact(t)
	artifactHash := sha256.Sum256(artifactData)
	manifestJSON, err := json.Marshal(manifest.Manifest{
		ID:            plugins.RemoteDesktopEnginePluginID,
		Name:          "Remote Desktop Engine",
		Version:       pluginVersion,
		Entry:         "remote-desktop-engine/engine",
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

	runtime := Config{
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

	startErr := module.Handle(context.Background(), startCmd)
	startResult := unwrapResult(t, startErr)
	if !startResult.Success {
		t.Fatalf("start command failed: %s", startResult.Error)
	}

	select {
	case req := <-handshakeCh:
		if strings.TrimSpace(req.PluginVersion) != pluginVersion {
			t.Fatalf("expected plugin version %s, got %q", pluginVersion, req.PluginVersion)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for negotiation request")
	}

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
	stopErr := module.Handle(context.Background(), stopCmd)
	stopResult := unwrapResult(t, stopErr)
	if !stopResult.Success {
		t.Fatalf("stop command failed: %s", stopResult.Error)
	}

	if err := module.Shutdown(context.Background()); err != nil {
		t.Fatalf("module shutdown: %v", err)
	}

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

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	projectRoot := filepath.Clean(filepath.Join(workDir, "..", ".."))

	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "engine")

	build := exec.Command("go", "build", "-o", binaryPath, "./cmd/remote-desktop-engine")
	build.Dir = projectRoot
	build.Env = os.Environ()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build plugin binary: %v: %s", err, out)
	}

	binaryData, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("read plugin binary: %v", err)
	}

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	header := &zip.FileHeader{Name: "remote-desktop-engine/engine", Method: zip.Deflate}
	header.SetMode(0o755)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatalf("create zip header: %v", err)
	}
	if _, err := entry.Write(binaryData); err != nil {
		t.Fatalf("write plugin binary: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buf.Bytes()
}

func unwrapResult(t *testing.T, err error) protocol.CommandResult {
	t.Helper()
	if err == nil {
		return protocol.CommandResult{}
	}
	var resultErr *CommandResultError
	if !errors.As(err, &resultErr) {
		t.Fatalf("unexpected error type: %T", err)
	}
	return resultErr.Result
}
