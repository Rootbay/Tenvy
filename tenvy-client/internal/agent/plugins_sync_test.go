package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	remotedesktop "github.com/rootbay/tenvy-client/internal/modules/control/remotedesktop"
	"github.com/rootbay/tenvy-client/internal/plugins"
	"github.com/rootbay/tenvy-client/internal/protocol"
	manifest "github.com/rootbay/tenvy-client/shared/pluginmanifest"
)

func TestApplyPluginManifestDeltaRemovesRemoteDesktopPlugin(t *testing.T) {
	t.Parallel()

	pluginRoot := t.TempDir()
	manager, err := plugins.NewManager(pluginRoot, log.New(io.Discard, "", 0), manifest.VerifyOptions{})
	if err != nil {
		t.Fatalf("new plugin manager: %v", err)
	}

	stagedDir := filepath.Join(manager.Root(), plugins.RemoteDesktopEnginePluginID)
	if err := os.MkdirAll(stagedDir, 0o755); err != nil {
		t.Fatalf("create plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stagedDir, "manifest.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stagedDir, ".status.json"), []byte(`{"status":"installed"}`), 0o644); err != nil {
		t.Fatalf("write status: %v", err)
	}

	agent := &Agent{
		id:       "agent-1",
		plugins:  manager,
		modules:  newDefaultModuleManager(),
		logger:   log.New(io.Discard, "", 0),
		metadata: protocol.AgentMetadata{OS: "windows", Architecture: "amd64", Version: "1.0.0"},
	}
	agent.setPluginManifestList(&manifest.ManifestList{
		Version: "1",
		Manifests: []manifest.ManifestDescriptor{
			{
				PluginID:       plugins.RemoteDesktopEnginePluginID,
				ManifestDigest: "digest-1",
				Version:        "1.0.0",
			},
		},
	})

	if err := agent.modules.RegisterModuleExtension("remote-desktop", ModuleExtension{
		Source:  plugins.RemoteDesktopEnginePluginID,
		Version: "1.0.0",
		Capabilities: []ModuleCapability{
			{ID: "remote-desktop.transport.quic", Name: "remote-desktop.transport.quic"},
		},
	}); err != nil {
		t.Fatalf("register extension: %v", err)
	}

	remote := agent.modules.remoteDesktopModule()
	if remote == nil {
		t.Fatal("remote desktop module not registered")
	}
	engine := &fakeRemoteDesktopEngine{}
	remote.mu.Lock()
	remote.engine = engine
	remote.requiredVersion = "1.0.0"
	remote.mu.Unlock()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(manifest.ManifestList{Version: "2"})
	}))
	defer server.Close()

	agent.baseURL = server.URL
	agent.client = server.Client()
	agent.buildVersion = "1.0.0"

	delta := &manifest.ManifestDelta{Version: "2", Removed: []string{plugins.RemoteDesktopEnginePluginID}}
	if err := agent.applyPluginManifestDelta(context.Background(), delta); err != nil {
		t.Fatalf("apply delta: %v", err)
	}

	if _, err := os.Stat(stagedDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected plugin directory removed, stat error = %v", err)
	}

	if snapshot := agent.plugins.Snapshot(); snapshot != nil {
		t.Fatalf("expected plugin snapshot to be nil, got %+v", snapshot)
	}

	remote.mu.Lock()
	currentEngine := remote.engine
	requiredVersion := remote.requiredVersion
	remote.mu.Unlock()

	if currentEngine == nil {
		t.Fatal("expected remote desktop engine to be reinitialized")
	}
	if _, ok := currentEngine.(*remotedesktop.RemoteDesktopStreamer); !ok {
		t.Fatalf("expected built-in streamer engine, got %T", currentEngine)
	}
	if requiredVersion != "" {
		t.Fatalf("expected required version cleared, got %q", requiredVersion)
	}
	if !engine.shutdownCalled {
		t.Fatal("expected previous engine to be shut down")
	}

	metadata := agent.modules.Metadata()
	var remoteMetadata *ModuleMetadata
	for index := range metadata {
		if metadata[index].ID == "remote-desktop" {
			remoteMetadata = &metadata[index]
			break
		}
	}
	if remoteMetadata == nil {
		t.Fatal("remote desktop metadata missing")
	}
	if len(remoteMetadata.Extensions) != 0 {
		t.Fatalf("expected no extensions, got %d", len(remoteMetadata.Extensions))
	}
	if len(remoteMetadata.Capabilities) != 2 {
		t.Fatalf("expected base capabilities preserved, got %d", len(remoteMetadata.Capabilities))
	}

	payload := agent.pluginSyncPayload()
	if payload == nil {
		t.Fatal("expected plugin telemetry payload")
	}
	if payload.Installations != nil && len(payload.Installations) > 0 {
		t.Fatalf("expected installations to be empty, got %+v", payload.Installations)
	}
	if payload.Manifests == nil {
		t.Fatal("expected manifest state to be present")
	}
	if payload.Manifests.Version != "2" {
		t.Fatalf("unexpected manifest version %q", payload.Manifests.Version)
	}
	if payload.Manifests.Digests != nil {
		if _, ok := payload.Manifests.Digests[plugins.RemoteDesktopEnginePluginID]; ok {
			t.Fatalf("expected remote desktop digest removed, got %+v", payload.Manifests.Digests)
		}
	}
}

func TestFetchApprovedPluginListUsesClientPluginEndpoint(t *testing.T) {
	t.Parallel()

	var (
		requestedPath  string
		receivedAuth   string
		receivedAccept string
		receivedAgent  string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")
		receivedAccept = r.Header.Get("Accept")
		receivedAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/vnd.tenvy.plugin-manifest+json")
		json.NewEncoder(w).Encode(manifest.ManifestList{Version: "1"})
	}))
	t.Cleanup(server.Close)

	agent := &Agent{
		id:           "agent-1",
		key:          "agent-key",
		baseURL:      server.URL,
		client:       server.Client(),
		buildVersion: "1.2.3",
	}

	snapshot, err := agent.fetchApprovedPluginList(context.Background())
	if err != nil {
		t.Fatalf("fetch approved plugin list: %v", err)
	}

	if snapshot == nil || snapshot.Version != "1" {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}

	if requestedPath != "/api/clients/agent-1/plugins" {
		t.Fatalf("unexpected request path %q", requestedPath)
	}

	if receivedAuth != "Bearer agent-key" {
		t.Fatalf("unexpected authorization header %q", receivedAuth)
	}

	if receivedAccept != "application/vnd.tenvy.plugin-manifest+json" {
		t.Fatalf("unexpected accept header %q", receivedAccept)
	}

	if receivedAgent == "" {
		t.Fatal("expected user agent header to be set")
	}
}

func TestStagePluginsFromListSkipsManualRemoteDesktopWithoutSignal(t *testing.T) {
	t.Parallel()

	pluginRoot := t.TempDir()
	manager, err := plugins.NewManager(pluginRoot, log.New(io.Discard, "", 0), manifest.VerifyOptions{})
	if err != nil {
		t.Fatalf("new plugin manager: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	agent := &Agent{
		id:       "agent-1",
		baseURL:  server.URL,
		client:   server.Client(),
		plugins:  manager,
		modules:  newDefaultModuleManager(),
		logger:   log.New(io.Discard, "", 0),
		metadata: protocol.AgentMetadata{OS: "windows", Architecture: "amd64", Version: "1.0.0"},
	}

	snapshot := &manifest.ManifestList{
		Version: "1",
		Manifests: []manifest.ManifestDescriptor{
			{
				PluginID:       plugins.RemoteDesktopEnginePluginID,
				ManifestDigest: "digest-1",
				Distribution: manifest.ManifestBriefing{
					DefaultMode: manifest.DeliveryManual,
					AutoUpdate:  false,
				},
			},
		},
	}

	if err := agent.stagePluginsFromList(context.Background(), snapshot); err != nil {
		t.Fatalf("stage plugins: %v", err)
	}

	if snapshot := agent.plugins.Snapshot(); snapshot != nil && len(snapshot.Installations) > 0 {
		t.Fatalf("expected no plugin installations recorded, got %#v", snapshot.Installations)
	}
}

func TestStagePluginsFromListStagesManualRemoteDesktopWhenRequested(t *testing.T) {
	t.Parallel()

	pluginID := plugins.RemoteDesktopEnginePluginID
	original := pluginStages.Lookup(pluginID)
	handler := &testPluginStageHandler{
		outcome: pluginStageOutcome{
			Manifest: &manifest.Manifest{
				ID:           pluginID,
				Version:      "2.0.0",
				Capabilities: []string{"remote-desktop.metrics"},
			},
			Staged: true,
		},
	}
	pluginStages.Register(pluginID, handler)
	t.Cleanup(func() {
		if original != nil {
			pluginStages.Register(pluginID, original)
		} else {
			pluginStages.Unregister(pluginID)
		}
	})

	pluginRoot := t.TempDir()
	manager, err := plugins.NewManager(pluginRoot, log.New(io.Discard, "", 0), manifest.VerifyOptions{})
	if err != nil {
		t.Fatalf("new plugin manager: %v", err)
	}

	agent := &Agent{
		id:       "agent-1",
		baseURL:  "https://controller.test",
		client:   &http.Client{},
		plugins:  manager,
		modules:  newDefaultModuleManager(),
		logger:   log.New(io.Discard, "", 0),
		metadata: protocol.AgentMetadata{OS: "windows", Architecture: "amd64", Version: "1.0.0"},
	}

	snapshot := &manifest.ManifestList{
		Version: "1",
		Manifests: []manifest.ManifestDescriptor{
			{
				PluginID:       pluginID,
				ManifestDigest: "digest-1",
				ManualPushAt:   "2024-01-02T03:04:05Z",
				Distribution: manifest.ManifestBriefing{
					DefaultMode: manifest.DeliveryManual,
					AutoUpdate:  false,
				},
			},
		},
	}

	if err := agent.stagePluginsFromList(context.Background(), snapshot); err != nil {
		t.Fatalf("stage plugins: %v", err)
	}
	if handler.calls != 1 {
		t.Fatalf("expected handler invoked once, got %d", handler.calls)
	}

	metadata := agent.modules.Metadata()
	var remoteMetadata *ModuleMetadata
	for index := range metadata {
		if strings.EqualFold(metadata[index].ID, "remote-desktop") {
			remoteMetadata = &metadata[index]
			break
		}
	}
	if remoteMetadata == nil {
		t.Fatal("remote desktop metadata missing")
	}

	var extensionFound bool
	for _, ext := range remoteMetadata.Extensions {
		if ext.Source != pluginID {
			continue
		}
		extensionFound = true
		if ext.Version != "2.0.0" {
			t.Fatalf("unexpected extension version %q", ext.Version)
		}
	}
	if !extensionFound {
		t.Fatalf("expected extension registered for %s", pluginID)
	}
}

func TestStagePluginsFromListRegistersCapabilitiesForCustomPlugin(t *testing.T) {
	t.Parallel()

	pluginID := "custom-plugin"
	handler := &testPluginStageHandler{
		outcome: pluginStageOutcome{
			Manifest: &manifest.Manifest{
				ID:           pluginID,
				Version:      "2.0.0",
				Capabilities: []string{"remote-desktop.metrics"},
			},
			Staged: true,
		},
	}
	pluginStages.Register(pluginID, handler)
	t.Cleanup(func() {
		pluginStages.Unregister(pluginID)
	})

	pluginRoot := t.TempDir()
	manager, err := plugins.NewManager(pluginRoot, log.New(io.Discard, "", 0), manifest.VerifyOptions{})
	if err != nil {
		t.Fatalf("new plugin manager: %v", err)
	}

	agent := &Agent{
		id:      "agent-1",
		plugins: manager,
		client:  &http.Client{},
		modules: newDefaultModuleManager(),
		logger:  log.New(io.Discard, "", 0),
	}

	snapshot := &manifest.ManifestList{
		Version: "3",
		Manifests: []manifest.ManifestDescriptor{
			{
				PluginID:       pluginID,
				ManifestDigest: "digest-123",
			},
		},
	}

	if err := agent.stagePluginsFromList(context.Background(), snapshot); err != nil {
		t.Fatalf("stage plugins: %v", err)
	}

	if handler.calls != 1 {
		t.Fatalf("expected handler to be invoked once, got %d", handler.calls)
	}

	metadata := agent.modules.Metadata()
	var remoteMetadata *ModuleMetadata
	for index := range metadata {
		if metadata[index].ID == "remote-desktop" {
			remoteMetadata = &metadata[index]
			break
		}
	}
	if remoteMetadata == nil {
		t.Fatal("remote desktop metadata missing")
	}

	found := false
	for _, ext := range remoteMetadata.Extensions {
		if ext.Source != pluginID {
			continue
		}
		found = true
		if ext.Version != "2.0.0" {
			t.Fatalf("unexpected extension version %q", ext.Version)
		}
		if len(ext.Capabilities) != 1 {
			t.Fatalf("expected one capability, got %d", len(ext.Capabilities))
		}
		if ext.Capabilities[0].ID != "remote-desktop.metrics" {
			t.Fatalf("unexpected capability id %q", ext.Capabilities[0].ID)
		}
	}
	if !found {
		t.Fatalf("expected extension registered for %s", pluginID)
	}
}

type testPluginStageHandler struct {
	outcome pluginStageOutcome
	err     error
	calls   int
}

func (h *testPluginStageHandler) Stage(ctx context.Context, agent *Agent, descriptor manifest.ManifestDescriptor) (pluginStageOutcome, error) {
	h.calls++
	return h.outcome, h.err
}
