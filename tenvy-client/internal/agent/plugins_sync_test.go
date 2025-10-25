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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	remotedesktop "github.com/rootbay/tenvy-client/internal/modules/control/remotedesktop"
	"github.com/rootbay/tenvy-client/internal/plugins"
	"github.com/rootbay/tenvy-client/internal/plugins/testsupport"
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

func TestActivatePluginLaunchesRuntime(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "started.txt")
	source := "package main\nimport (\n\t\"os\"\n\t\"time\"\n)\nfunc main() {\n\tmarker := os.Getenv(\"PLUGIN_TEST_MARKER\")\n\tos.WriteFile(marker, []byte(\"started\"), 0o644)\n\tfor {\n\t\ttime.Sleep(10 * time.Millisecond)\n\t}\n}\n"
	binary := buildPluginBinary(t, source)

	agent := &Agent{
		modules: newModuleManager(),
		logger:  log.New(io.Discard, "", 0),
	}

	mf := manifest.Manifest{ID: "test-plugin", Version: "1.0.0"}
	t.Setenv("PLUGIN_TEST_MARKER", marker)

	if err := agent.activatePlugin(context.Background(), mf, binary, ""); err != nil {
		t.Fatalf("activate plugin: %v", err)
	}
	t.Cleanup(func() {
		_ = agent.modules.DeactivatePlugin(context.Background(), "test-plugin")
	})

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("plugin did not create marker file %s", marker)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if agent.modules.PluginHandle("test-plugin") == nil {
		t.Fatal("expected plugin handle to be registered")
	}
}

func TestActivatePluginLaunchesWasmRuntime(t *testing.T) {
	modulePath := buildWasmModule(t)

	agent := &Agent{
		modules: newModuleManager(),
		logger:  log.New(io.Discard, "", 0),
	}

	mf := manifest.Manifest{
		ID:      "wasm-plugin",
		Version: "1.0.0",
		Runtime: &manifest.RuntimeDescriptor{
			Type:      manifest.RuntimeWASM,
			Sandboxed: true,
			Host: &manifest.RuntimeHostContract{
				Interfaces: []string{manifest.HostInterfaceCoreV1},
				APIVersion: "1.0",
			},
		},
	}

	if err := agent.activatePlugin(context.Background(), mf, modulePath, ""); err != nil {
		t.Fatalf("activate wasm plugin: %v", err)
	}
	t.Cleanup(func() {
		_ = agent.modules.DeactivatePlugin(context.Background(), mf.ID)
	})

	if agent.modules.PluginHandle(mf.ID) == nil {
		t.Fatal("expected wasm plugin handle to be registered")
	}
}

func TestActivatePluginRecordsRuntimeFailure(t *testing.T) {
	t.Parallel()

	logger := log.New(io.Discard, "", 0)
	manager, err := plugins.NewManager(t.TempDir(), logger, manifest.VerifyOptions{})
	if err != nil {
		t.Fatalf("new plugin manager: %v", err)
	}

	agent := &Agent{
		modules: newModuleManager(),
		plugins: manager,
		logger:  logger,
	}

	mf := manifest.Manifest{ID: "broken-plugin", Version: "2.0.0"}
	missing := filepath.Join(t.TempDir(), "missing.exe")

	if err := agent.activatePlugin(context.Background(), mf, missing, ""); err == nil {
		t.Fatal("expected activation to fail for missing binary")
	}

	statusPath := filepath.Join(manager.Root(), "broken-plugin", ".status.json")
	data, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("read status file: %v", err)
	}

	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("unmarshal status file: %v", err)
	}
	statusValue, ok := record["status"].(string)
	if !ok {
		t.Fatalf("status field missing from record: %+v", record)
	}
	if !strings.EqualFold(statusValue, string(manifest.InstallError)) {
		t.Fatalf("expected install error status, got %v", statusValue)
	}
	errValue, _ := record["error"].(string)
	if strings.TrimSpace(errValue) == "" {
		t.Fatalf("expected error message in status file, got %+v", record)
	}
}

func TestActivatePluginRestoresBackupOnRuntimeFailure(t *testing.T) {
	t.Parallel()

	logger := log.New(io.Discard, "", 0)
	pluginRoot := t.TempDir()
	manager, err := plugins.NewManager(pluginRoot, logger, manifest.VerifyOptions{})
	if err != nil {
		t.Fatalf("new plugin manager: %v", err)
	}

	pluginID := "runtime-plugin"
	pluginDir := filepath.Join(pluginRoot, pluginID)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("create plugin directory: %v", err)
	}
	entryPath := filepath.Join(pluginDir, "plugin.bin")
	if err := os.WriteFile(entryPath, []byte("new payload"), 0o644); err != nil {
		t.Fatalf("write plugin entry: %v", err)
	}

	backupDir, err := os.MkdirTemp(pluginRoot, pluginID+"-backup-")
	if err != nil {
		t.Fatalf("create backup directory: %v", err)
	}
	oldEntry := filepath.Join(backupDir, "previous.bin")
	if err := os.WriteFile(oldEntry, []byte("previous payload"), 0o644); err != nil {
		t.Fatalf("write backup entry: %v", err)
	}

	agent := &Agent{
		modules: newModuleManager(),
		plugins: manager,
		logger:  logger,
	}

	mf := manifest.Manifest{ID: pluginID, Version: "2.0.0"}
	err = agent.activatePlugin(context.Background(), mf, entryPath, backupDir)
	if err == nil {
		t.Fatal("expected runtime launch to fail")
	}
	if !strings.Contains(err.Error(), "previous version restored") {
		t.Fatalf("expected error message to mention restore, got %v", err)
	}

	restoredEntry := filepath.Join(pluginDir, "previous.bin")
	data, readErr := os.ReadFile(restoredEntry)
	if readErr != nil {
		t.Fatalf("read restored entry: %v", readErr)
	}
	if string(data) != "previous payload" {
		t.Fatalf("unexpected restored payload %q", string(data))
	}

	if _, statErr := os.Stat(backupDir); statErr == nil {
		t.Fatalf("expected backup directory to be removed after restore")
	}

	statusPath := filepath.Join(pluginDir, ".status.json")
	statusData, readStatusErr := os.ReadFile(statusPath)
	if readStatusErr != nil {
		t.Fatalf("read status file: %v", readStatusErr)
	}
	if !strings.Contains(string(statusData), "previous version restored") {
		t.Fatalf("expected status message to mention restore, got %s", statusData)
	}
}

func TestActivatePluginRestoresBackupOnModuleFailure(t *testing.T) {
	t.Parallel()

	logger := log.New(io.Discard, "", 0)
	pluginRoot := t.TempDir()
	manager, err := plugins.NewManager(pluginRoot, logger, manifest.VerifyOptions{})
	if err != nil {
		t.Fatalf("new plugin manager: %v", err)
	}

	pluginID := "module-plugin"
	pluginDir := filepath.Join(pluginRoot, pluginID)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("create plugin directory: %v", err)
	}

	source := "package main\nimport \"time\"\nfunc main() { for { time.Sleep(10 * time.Millisecond) } }\n"
	binary := buildPluginBinary(t, source)
	entryPath := filepath.Join(pluginDir, "plugin.bin")
	if err := os.Rename(binary, entryPath); err != nil {
		t.Fatalf("place plugin binary: %v", err)
	}

	backupDir, err := os.MkdirTemp(pluginRoot, pluginID+"-backup-")
	if err != nil {
		t.Fatalf("create backup directory: %v", err)
	}
	oldEntry := filepath.Join(backupDir, "previous.bin")
	if err := os.WriteFile(oldEntry, []byte("previous payload"), 0o644); err != nil {
		t.Fatalf("write backup entry: %v", err)
	}

	agent := &Agent{
		modules: newModuleManager(),
		plugins: manager,
		logger:  logger,
	}

	mf := manifest.Manifest{ID: pluginID, Version: "3.0.0", Capabilities: []string{"remote-desktop.stream"}}
	err = agent.activatePlugin(context.Background(), mf, entryPath, backupDir)
	if err == nil {
		t.Fatal("expected module activation to fail")
	}
	if !strings.Contains(err.Error(), "previous version restored") {
		t.Fatalf("expected error message to mention restore, got %v", err)
	}

	restoredEntry := filepath.Join(pluginDir, "previous.bin")
	data, readErr := os.ReadFile(restoredEntry)
	if readErr != nil {
		t.Fatalf("read restored entry: %v", readErr)
	}
	if string(data) != "previous payload" {
		t.Fatalf("unexpected restored payload %q", string(data))
	}

	if _, statErr := os.Stat(backupDir); statErr == nil {
		t.Fatalf("expected backup directory to be removed after restore")
	}

	statusPath := filepath.Join(pluginDir, ".status.json")
	statusData, readStatusErr := os.ReadFile(statusPath)
	if readStatusErr != nil {
		t.Fatalf("read status file: %v", readStatusErr)
	}
	if !strings.Contains(string(statusData), "previous version restored") {
		t.Fatalf("expected status message to mention restore, got %s", statusData)
	}
}

func buildPluginBinary(t *testing.T, source string) string {
	t.Helper()

	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte(source), 0o644); err != nil {
		t.Fatalf("write plugin source: %v", err)
	}

	binary := filepath.Join(dir, "plugin")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", binary, src)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		t.Fatalf("build plugin: %v", err)
	}

	return binary
}

func copyExecutable(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read executable: %v", err)
	}
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
}

func buildWasmModule(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "plugin.wasm")
	if err := os.WriteFile(path, testsupport.SandboxModule, 0o644); err != nil {
		t.Fatalf("write wasm module: %v", err)
	}
	return path
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
	pluginRoot := t.TempDir()
	manager, err := plugins.NewManager(pluginRoot, log.New(io.Discard, "", 0), manifest.VerifyOptions{})
	if err != nil {
		t.Fatalf("new plugin manager: %v", err)
	}

	entryPath := filepath.Join(pluginRoot, pluginID, "engine.bin")
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o755); err != nil {
		t.Fatalf("create entry dir: %v", err)
	}
	source := "package main\nimport \"time\"\nfunc main() { for { time.Sleep(10 * time.Millisecond) } }\n"
	binary := buildPluginBinary(t, source)
	if err := os.Rename(binary, entryPath); err != nil {
		t.Fatalf("place entry binary: %v", err)
	}

	handler := &testPluginStageHandler{
		outcome: pluginStageOutcome{
			Manifest: &manifest.Manifest{
				ID:           pluginID,
				Version:      "2.0.0",
				Capabilities: []string{"remote-desktop.metrics"},
			},
			EntryPath: entryPath,
			Staged:    true,
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

	agent := &Agent{
		id:       "agent-1",
		baseURL:  "https://controller.test",
		client:   &http.Client{},
		plugins:  manager,
		modules:  newDefaultModuleManager(),
		logger:   log.New(io.Discard, "", 0),
		metadata: protocol.AgentMetadata{OS: "windows", Architecture: "amd64", Version: "1.0.0"},
	}

	t.Cleanup(func() {
		_ = agent.modules.DeactivatePlugin(context.Background(), pluginID)
	})

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
	pluginRoot := t.TempDir()
	manager, err := plugins.NewManager(pluginRoot, log.New(io.Discard, "", 0), manifest.VerifyOptions{})
	if err != nil {
		t.Fatalf("new plugin manager: %v", err)
	}

	entryPath := filepath.Join(pluginRoot, pluginID, "plugin.bin")
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o755); err != nil {
		t.Fatalf("create entry dir: %v", err)
	}
	source := "package main\nimport \"time\"\nfunc main() { for { time.Sleep(10 * time.Millisecond) } }\n"
	binary := buildPluginBinary(t, source)
	if err := os.Rename(binary, entryPath); err != nil {
		t.Fatalf("place entry binary: %v", err)
	}

	handler := &testPluginStageHandler{
		outcome: pluginStageOutcome{
			Manifest: &manifest.Manifest{
				ID:           pluginID,
				Version:      "2.0.0",
				Capabilities: []string{"remote-desktop.metrics"},
			},
			EntryPath: entryPath,
			Staged:    true,
		},
	}
	pluginStages.Register(pluginID, handler)
	t.Cleanup(func() {
		pluginStages.Unregister(pluginID)
	})

	agent := &Agent{
		id:      "agent-1",
		plugins: manager,
		client:  &http.Client{},
		modules: newDefaultModuleManager(),
		logger:  log.New(io.Discard, "", 0),
	}

	t.Cleanup(func() {
		_ = agent.modules.DeactivatePlugin(context.Background(), pluginID)
	})

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

func TestStagePluginsFromListDefersConflictingDescriptors(t *testing.T) {
	t.Parallel()

	pluginID := "conflict-plugin"
	pluginRoot := t.TempDir()
	manager, err := plugins.NewManager(pluginRoot, log.New(io.Discard, "", 0), manifest.VerifyOptions{})
	if err != nil {
		t.Fatalf("new plugin manager: %v", err)
	}

	handler := &testPluginStageHandler{}
	pluginStages.Register(pluginID, handler)
	t.Cleanup(func() {
		pluginStages.Unregister(pluginID)
	})

	agent := &Agent{
		id:      "agent-1",
		plugins: manager,
		client:  &http.Client{},
		modules: newDefaultModuleManager(),
		logger:  log.New(io.Discard, "", 0),
	}

	snapshot := &manifest.ManifestList{
		Version: "4",
		Manifests: []manifest.ManifestDescriptor{
			{
				PluginID:       pluginID,
				ManifestDigest: "digest-1",
				Version:        "1.2.3",
			},
			{
				PluginID:       pluginID,
				ManifestDigest: "digest-2",
				Version:        "1.4.0",
			},
		},
	}

	err = agent.stagePluginsFromList(context.Background(), snapshot)
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !strings.Contains(err.Error(), "manifest conflict") {
		t.Fatalf("expected conflict error, got %v", err)
	}
	if handler.calls != 0 {
		t.Fatalf("expected handler to be skipped, invoked %d times", handler.calls)
	}

	statusPath := filepath.Join(pluginRoot, pluginID, ".status.json")
	data, readErr := os.ReadFile(statusPath)
	if readErr != nil {
		t.Fatalf("read status: %v", readErr)
	}

	var status struct {
		Version string `json:"version"`
		State   string `json:"status"`
		Error   string `json:"error"`
	}
	if decodeErr := json.Unmarshal(data, &status); decodeErr != nil {
		t.Fatalf("decode status: %v", decodeErr)
	}
	if !strings.EqualFold(status.State, string(manifest.InstallBlocked)) {
		t.Fatalf("expected status blocked, got %q", status.State)
	}
	if status.Version != "1.4.0" {
		t.Fatalf("expected preferred version recorded, got %q", status.Version)
	}
	if !strings.Contains(status.Error, "conflicting manifests") {
		t.Fatalf("expected conflict message, got %q", status.Error)
	}
}

func TestStagePluginsFromListOrdersDependencies(t *testing.T) {
	t.Parallel()

	pluginRoot := t.TempDir()
	manager, err := plugins.NewManager(pluginRoot, log.New(io.Discard, "", 0), manifest.VerifyOptions{})
	if err != nil {
		t.Fatalf("new plugin manager: %v", err)
	}

	agent := &Agent{
		id:      "agent-1",
		baseURL: "https://controller.test",
		client:  &http.Client{},
		plugins: manager,
		modules: newDefaultModuleManager(),
		logger:  log.New(io.Discard, "", 0),
		metadata: protocol.AgentMetadata{
			OS:           runtime.GOOS,
			Architecture: runtime.GOARCH,
			Version:      "1.0.0",
		},
	}

	source := "package main\nimport \"time\"\nfunc main() { for { time.Sleep(10 * time.Millisecond) } }\n"
	binary := buildPluginBinary(t, source)

	calls := make([]string, 0, 2)

	registerHandler := func(id string) {
		entryDir := filepath.Join(pluginRoot, id)
		if err := os.MkdirAll(entryDir, 0o755); err != nil {
			t.Fatalf("create entry dir: %v", err)
		}
		entryPath := filepath.Join(entryDir, "plugin.bin")
		copyExecutable(t, binary, entryPath)

		handler := &recordingStageHandler{id: id, entryPath: entryPath, calls: &calls}
		original := pluginStages.Lookup(id)
		pluginStages.Register(id, handler)
		t.Cleanup(func() {
			if original != nil {
				pluginStages.Register(id, original)
			} else {
				pluginStages.Unregister(id)
			}
			_ = agent.modules.DeactivatePlugin(context.Background(), id)
		})
	}

	registerHandler("alpha")
	registerHandler("beta")

	snapshot := &manifest.ManifestList{
		Version: "1",
		Manifests: []manifest.ManifestDescriptor{
			{
				PluginID:     "beta",
				Version:      "1.0.0",
				Dependencies: []string{"alpha"},
			},
			{
				PluginID: "alpha",
				Version:  "1.0.0",
			},
		},
	}

	if err := agent.stagePluginsFromList(context.Background(), snapshot); err != nil {
		t.Fatalf("stage plugins: %v", err)
	}

	if len(calls) != 2 || calls[0] != "alpha" || calls[1] != "beta" {
		t.Fatalf("expected staging order [alpha beta], got %v", calls)
	}
}

func TestStagePluginsFromListBlocksMissingDependencies(t *testing.T) {
	t.Parallel()

	pluginRoot := t.TempDir()
	manager, err := plugins.NewManager(pluginRoot, log.New(io.Discard, "", 0), manifest.VerifyOptions{})
	if err != nil {
		t.Fatalf("new plugin manager: %v", err)
	}

	handler := &testPluginStageHandler{}
	pluginStages.Register("beta", handler)
	t.Cleanup(func() { pluginStages.Unregister("beta") })

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
				PluginID:     "beta",
				Version:      "1.0.0",
				Dependencies: []string{"alpha"},
			},
		},
	}

	err = agent.stagePluginsFromList(context.Background(), snapshot)
	if err == nil || !strings.Contains(err.Error(), "missing dependencies") {
		t.Fatalf("expected missing dependency error, got %v", err)
	}
	if handler.calls != 0 {
		t.Fatalf("expected handler not invoked, got %d", handler.calls)
	}

	statusPath := filepath.Join(pluginRoot, "beta", ".status.json")
	data, readErr := os.ReadFile(statusPath)
	if readErr != nil {
		t.Fatalf("read status: %v", readErr)
	}

	var status struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if decodeErr := json.Unmarshal(data, &status); decodeErr != nil {
		t.Fatalf("decode status: %v", decodeErr)
	}
	if status.Status != string(manifest.InstallBlocked) {
		t.Fatalf("expected status blocked, got %q", status.Status)
	}
	if !strings.Contains(status.Error, "missing dependencies") {
		t.Fatalf("expected missing dependency message, got %q", status.Error)
	}
}

func TestStagePluginsFromListDetectsDependencyCycles(t *testing.T) {
	t.Parallel()

	pluginRoot := t.TempDir()
	manager, err := plugins.NewManager(pluginRoot, log.New(io.Discard, "", 0), manifest.VerifyOptions{})
	if err != nil {
		t.Fatalf("new plugin manager: %v", err)
	}

	alphaHandler := &testPluginStageHandler{}
	betaHandler := &testPluginStageHandler{}
	pluginStages.Register("alpha", alphaHandler)
	pluginStages.Register("beta", betaHandler)
	t.Cleanup(func() {
		pluginStages.Unregister("alpha")
		pluginStages.Unregister("beta")
	})

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
				PluginID:     "alpha",
				Version:      "1.0.0",
				Dependencies: []string{"beta"},
			},
			{
				PluginID:     "beta",
				Version:      "1.0.0",
				Dependencies: []string{"alpha"},
			},
		},
	}

	err = agent.stagePluginsFromList(context.Background(), snapshot)
	if err == nil || !strings.Contains(err.Error(), "dependency cycle") {
		t.Fatalf("expected dependency cycle error, got %v", err)
	}
	if alphaHandler.calls != 0 || betaHandler.calls != 0 {
		t.Fatalf("expected handlers skipped, got alpha=%d beta=%d", alphaHandler.calls, betaHandler.calls)
	}

	for _, id := range []string{"alpha", "beta"} {
		statusPath := filepath.Join(pluginRoot, id, ".status.json")
		data, readErr := os.ReadFile(statusPath)
		if readErr != nil {
			t.Fatalf("read status for %s: %v", id, readErr)
		}
		var status struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		}
		if decodeErr := json.Unmarshal(data, &status); decodeErr != nil {
			t.Fatalf("decode status for %s: %v", id, decodeErr)
		}
		if status.Status != string(manifest.InstallBlocked) {
			t.Fatalf("expected blocked status for %s, got %q", id, status.Status)
		}
		if !strings.Contains(status.Error, "dependency cycle") {
			t.Fatalf("expected cycle message for %s, got %q", id, status.Error)
		}
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

type recordingStageHandler struct {
	id        string
	entryPath string
	calls     *[]string
}

func (h *recordingStageHandler) Stage(ctx context.Context, agent *Agent, descriptor manifest.ManifestDescriptor) (pluginStageOutcome, error) {
	if h.calls != nil {
		*h.calls = append(*h.calls, h.id)
	}
	mf := &manifest.Manifest{ID: h.id, Version: strings.TrimSpace(descriptor.Version)}
	return pluginStageOutcome{Manifest: mf, EntryPath: h.entryPath, Staged: true}, nil
}

func TestBuildModuleExtensionsIncludesTelemetry(t *testing.T) {
	t.Parallel()

	mf := manifest.Manifest{
		ID:           "test-plugin",
		Version:      "1.0.0",
		Capabilities: []string{"remote-desktop.stream"},
		Telemetry:    []string{"remote-desktop.metrics"},
	}

	extensions := buildModuleExtensions(mf)
	if len(extensions) != 1 {
		t.Fatalf("expected single module extension, got %d", len(extensions))
	}

	ext, ok := extensions["remote-desktop"]
	if !ok {
		t.Fatalf("expected remote-desktop extension")
	}
	if ext.Source != "test-plugin" {
		t.Fatalf("unexpected extension source %q", ext.Source)
	}
	if ext.Version != "1.0.0" {
		t.Fatalf("unexpected extension version %q", ext.Version)
	}
	if len(ext.Telemetry) != 1 {
		t.Fatalf("expected telemetry descriptor, got %d", len(ext.Telemetry))
	}
	descriptor := ext.Telemetry[0]
	if descriptor.ID != "remote-desktop.metrics" {
		t.Fatalf("unexpected telemetry id %q", descriptor.ID)
	}
	if descriptor.Name == "" {
		t.Fatal("expected telemetry descriptor name populated")
	}
}
