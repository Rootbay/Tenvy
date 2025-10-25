package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	remotedesktop "github.com/rootbay/tenvy-client/internal/modules/control/remotedesktop"
	"github.com/rootbay/tenvy-client/internal/protocol"
)

type stubModule struct {
	metadata         ModuleMetadata
	initErr          error
	updateErr        error
	handleFunc       func(context.Context, protocol.Command) error
	initCalled       int
	updateCalled     int
	shutdownCalled   int
	handleCalled     int
	log              *[]string
	acceptExtensions bool
	registerErr      error
	registered       []ModuleExtension
	unregisterErr    error
	unregistered     []string
}

func (m *stubModule) Metadata() ModuleMetadata {
	return m.metadata
}

func (m *stubModule) ID() string {
	return m.metadata.ID
}

func (m *stubModule) Init(context.Context, Config) error {
	m.initCalled++
	if m.log != nil {
		*m.log = append(*m.log, m.metadata.ID+":init")
	}
	return m.initErr
}

func (m *stubModule) Handle(ctx context.Context, cmd protocol.Command) error {
	m.handleCalled++
	if m.log != nil {
		*m.log = append(*m.log, m.metadata.ID+":handle"+":"+cmd.Name)
	}
	if m.handleFunc != nil {
		return m.handleFunc(ctx, cmd)
	}
	return WrapCommandResult(protocol.CommandResult{CommandID: cmd.ID, Success: true, CompletedAt: "now"})
}

func (m *stubModule) UpdateConfig(Config) error {
	m.updateCalled++
	if m.log != nil {
		*m.log = append(*m.log, m.metadata.ID+":update")
	}
	return m.updateErr
}

func (m *stubModule) Shutdown(context.Context) error {
	m.shutdownCalled++
	if m.log != nil {
		*m.log = append(*m.log, m.metadata.ID+":shutdown")
	}
	return nil
}

func (m *stubModule) RegisterExtension(extension ModuleExtension) error {
	if !m.acceptExtensions {
		return m.registerErr
	}
	m.registered = append(m.registered, copyModuleExtension(extension))
	return m.registerErr
}

func (m *stubModule) UnregisterExtension(source string) error {
	if !m.acceptExtensions {
		return m.unregisterErr
	}
	m.unregistered = append(m.unregistered, strings.TrimSpace(source))
	return m.unregisterErr
}

func TestModuleManagerLifecycle(t *testing.T) {
	t.Parallel()

	var callLog []string
	moduleA := &stubModule{
		metadata: ModuleMetadata{
			ID:       "module-a",
			Title:    "Module A",
			Commands: []string{"alpha"},
		},
		log: &callLog,
	}

	moduleB := &stubModule{
		metadata: ModuleMetadata{
			ID:       "module-b",
			Title:    "Module B",
			Commands: []string{"beta"},
		},
		updateErr: errors.New("boom"),
		handleFunc: func(_ context.Context, cmd protocol.Command) error {
			return WrapCommandResult(protocol.CommandResult{CommandID: cmd.ID, Success: true, Output: "ok", CompletedAt: "done"})
		},
		log: &callLog,
	}

	manager := newModuleManager()
	manager.register(moduleA)
	manager.register(moduleB)

	runtime := Config{AgentID: "agent-123"}
	if err := manager.Init(context.Background(), runtime); err != nil {
		t.Fatalf("Init returned unexpected error: %v", err)
	}

	if moduleA.initCalled != 1 || moduleB.initCalled != 1 {
		t.Fatalf("expected init to be called once on each module, got %d and %d", moduleA.initCalled, moduleB.initCalled)
	}

	err := manager.UpdateConfig(runtime)
	if err == nil {
		t.Fatal("expected UpdateConfig to return aggregated error")
	}
	if !strings.Contains(err.Error(), moduleB.metadata.Title) {
		t.Fatalf("expected UpdateConfig error to reference module title, got %q", err)
	}

	if moduleA.updateCalled != 1 || moduleB.updateCalled != 1 {
		t.Fatalf("expected update to be called once on each module, got %d and %d", moduleA.updateCalled, moduleB.updateCalled)
	}

	handled, result := manager.HandleCommand(context.Background(), protocol.Command{ID: "cmd-1", Name: "beta"})
	if !handled {
		t.Fatal("expected command to be handled")
	}
	if result.CommandID != "cmd-1" || !result.Success || result.Output != "ok" {
		t.Fatalf("unexpected command result: %+v", result)
	}

	if handled, _ := manager.HandleCommand(context.Background(), protocol.Command{Name: "unknown"}); handled {
		t.Fatal("expected unknown command to be unhandled")
	}

	if err := manager.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned unexpected error: %v", err)
	}

	if moduleA.shutdownCalled != 1 || moduleB.shutdownCalled != 1 {
		t.Fatalf("expected shutdown to be called once on each module, got %d and %d", moduleA.shutdownCalled, moduleB.shutdownCalled)
	}

	expectedLog := []string{
		"module-a:init",
		"module-b:init",
		"module-a:update",
		"module-b:update",
		"module-b:handle:beta",
		"module-b:shutdown",
		"module-a:shutdown",
	}

	if len(callLog) != len(expectedLog) {
		t.Fatalf("call log length mismatch: got %v want %v", callLog, expectedLog)
	}
	for i := range expectedLog {
		if callLog[i] != expectedLog[i] {
			t.Fatalf("call log mismatch at %d: got %q want %q", i, callLog[i], expectedLog[i])
		}
	}

	metadata := manager.Metadata()
	if len(metadata) != 2 {
		t.Fatalf("expected metadata for two modules, got %d", len(metadata))
	}
	if metadata[0].ID != "module-a" || metadata[1].ID != "module-b" {
		t.Fatalf("unexpected metadata ordering: %+v", metadata)
	}
}

func TestModuleManagerSetEnabledModules(t *testing.T) {
	t.Parallel()

	moduleA := &stubModule{
		metadata: ModuleMetadata{
			ID:       "module-a",
			Title:    "Module A",
			Commands: []string{"alpha"},
		},
	}

	moduleB := &stubModule{
		metadata: ModuleMetadata{
			ID:       "module-b",
			Title:    "Module B",
			Commands: []string{"beta"},
		},
	}

	manager := newModuleManager()
	manager.register(moduleA)
	manager.register(moduleB)

	manager.SetEnabledModules([]string{"module-b"})

	runtime := Config{AgentID: "agent-456"}
	if err := manager.Init(context.Background(), runtime); err != nil {
		t.Fatalf("Init returned unexpected error: %v", err)
	}

	if moduleA.initCalled != 0 {
		t.Fatalf("expected module A to remain disabled, init called %d times", moduleA.initCalled)
	}
	if moduleB.initCalled != 1 {
		t.Fatalf("expected module B to initialize once, got %d", moduleB.initCalled)
	}

	metadata := manager.Metadata()
	if len(metadata) != 1 || metadata[0].ID != "module-b" {
		t.Fatalf("expected only enabled module metadata, got %+v", metadata)
	}

	if handled, _ := manager.HandleCommand(context.Background(), protocol.Command{Name: "alpha"}); handled {
		t.Fatal("expected disabled module command to be ignored")
	}

	if err := manager.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned unexpected error: %v", err)
	}

	if moduleA.shutdownCalled != 0 {
		t.Fatalf("expected disabled module shutdown to be skipped, got %d", moduleA.shutdownCalled)
	}
	if moduleB.shutdownCalled != 1 {
		t.Fatalf("expected enabled module shutdown once, got %d", moduleB.shutdownCalled)
	}

	manager.SetEnabledModules(nil)
	metadata = manager.Metadata()
	if len(metadata) != 2 {
		t.Fatalf("expected re-enabled metadata to include both modules, got %d", len(metadata))
	}
}

func TestModuleManagerRemoteModuleDisabled(t *testing.T) {
	t.Parallel()

	manager := newDefaultModuleManager()
	manager.SetEnabledModules([]string{"system-info"})

	if remote := manager.remoteDesktopModule(); remote != nil {
		t.Fatal("expected remote desktop module to be disabled")
	}

	metadata := manager.Metadata()
	for _, entry := range metadata {
		if strings.EqualFold(entry.ID, "remote-desktop") {
			t.Fatalf("expected remote desktop metadata to be filtered out: %+v", metadata)
		}
	}
}

func TestModuleManagerRegisterExtension(t *testing.T) {
	t.Parallel()

	module := &stubModule{
		metadata: ModuleMetadata{
			ID:           "ext-module",
			Title:        "Extension Module",
			Commands:     []string{"ext.command"},
			Capabilities: []ModuleCapability{{ID: "base.capability", Name: "base.capability"}},
		},
		acceptExtensions: true,
	}

	manager := newModuleManager()
	manager.register(module)

	err := manager.RegisterModuleExtension("ext-module", ModuleExtension{
		Source:  "plugin.remote",
		Version: "1.0.0",
		Capabilities: []ModuleCapability{
			{ID: " ext.capability ", Name: " ext.capability ", Description: " Extended feature "},
			{Name: "   "},
		},
	})
	if err != nil {
		t.Fatalf("register extension: %v", err)
	}

	metadata := manager.Metadata()
	if len(metadata) != 1 {
		t.Fatalf("expected single module metadata entry, got %d", len(metadata))
	}
	entry := metadata[0]
	if entry.ID != "ext-module" {
		t.Fatalf("unexpected metadata id %s", entry.ID)
	}
	if len(entry.Capabilities) != 2 {
		t.Fatalf("expected base + extension capabilities, got %d", len(entry.Capabilities))
	}
	if entry.Capabilities[1].ID != "ext.capability" {
		t.Fatalf("expected sanitized capability id, got %q", entry.Capabilities[1].ID)
	}
	if entry.Capabilities[1].Name != "ext.capability" {
		t.Fatalf("expected sanitized capability name, got %q", entry.Capabilities[1].Name)
	}
	if entry.Capabilities[1].Description != "Extended feature" {
		t.Fatalf("expected trimmed capability description, got %q", entry.Capabilities[1].Description)
	}
	if len(entry.Extensions) != 1 {
		t.Fatalf("expected metadata extension entry, got %d", len(entry.Extensions))
	}
	ext := entry.Extensions[0]
	if ext.Source != "plugin.remote" {
		t.Fatalf("unexpected extension source %s", ext.Source)
	}
	if ext.Version != "1.0.0" {
		t.Fatalf("unexpected extension version %s", ext.Version)
	}
	if len(ext.Capabilities) != 1 {
		t.Fatalf("expected sanitized extension capability list, got %d", len(ext.Capabilities))
	}
	if ext.Capabilities[0].ID != "ext.capability" {
		t.Fatalf("unexpected extension capability id %s", ext.Capabilities[0].ID)
	}
	if ext.Capabilities[0].Name != "ext.capability" {
		t.Fatalf("unexpected extension capability name %s", ext.Capabilities[0].Name)
	}
	if len(module.registered) != 1 {
		t.Fatalf("expected module registrar to receive extension, got %d", len(module.registered))
	}
	if module.registered[0].Source != "plugin.remote" {
		t.Fatalf("unexpected registrar extension source %s", module.registered[0].Source)
	}
}

func TestModuleManagerUnregisterExtension(t *testing.T) {
	t.Parallel()

	module := &stubModule{
		metadata: ModuleMetadata{
			ID:           "ext-module",
			Title:        "Extension Module",
			Commands:     []string{"ext.command"},
			Capabilities: []ModuleCapability{{ID: "base.capability", Name: "base.capability"}},
		},
		acceptExtensions: true,
	}

	manager := newModuleManager()
	manager.register(module)

	extension := ModuleExtension{
		Source:  "plugin.remote",
		Version: "1.0.0",
		Capabilities: []ModuleCapability{
			{ID: "ext.capability", Name: "ext.capability"},
		},
	}
	if err := manager.RegisterModuleExtension("ext-module", extension); err != nil {
		t.Fatalf("register extension: %v", err)
	}

	if err := manager.UnregisterModuleExtension("ext-module", "plugin.remote"); err != nil {
		t.Fatalf("unregister extension: %v", err)
	}

	metadata := manager.Metadata()
	if len(metadata) != 1 {
		t.Fatalf("expected metadata for one module, got %d", len(metadata))
	}
	entry := metadata[0]
	if entry.ID != "ext-module" {
		t.Fatalf("unexpected metadata id %s", entry.ID)
	}
	if len(entry.Extensions) != 0 {
		t.Fatalf("expected no metadata extensions, got %d", len(entry.Extensions))
	}
	if len(entry.Capabilities) != 1 {
		t.Fatalf("expected base capability only, got %d", len(entry.Capabilities))
	}
	if entry.Capabilities[0].ID != "base.capability" {
		t.Fatalf("unexpected capability id %s", entry.Capabilities[0].ID)
	}

	if len(module.unregistered) != 1 {
		t.Fatalf("expected registrar to receive unregister call, got %d", len(module.unregistered))
	}
	if module.unregistered[0] != "plugin.remote" {
		t.Fatalf("unexpected unregister source %s", module.unregistered[0])
	}
}

type fakeRemoteDesktopEngine struct {
	configureCalls []remotedesktop.Config
	startCalls     []remotedesktop.RemoteDesktopCommandPayload
	stopCalls      []string
	updateCalls    []remotedesktop.RemoteDesktopCommandPayload
	inputCalls     []remotedesktop.RemoteDesktopCommandPayload
	deliverCalls   []remotedesktop.RemoteDesktopFramePacket
	shutdownCalled bool

	configureErr error
	startErr     error
	stopErr      error
	updateErr    error
	inputErr     error
	deliverErr   error
}

func (f *fakeRemoteDesktopEngine) Configure(cfg remotedesktop.Config) error {
	f.configureCalls = append(f.configureCalls, cfg)
	return f.configureErr
}

func (f *fakeRemoteDesktopEngine) StartSession(ctx context.Context, payload remotedesktop.RemoteDesktopCommandPayload) error {
	f.startCalls = append(f.startCalls, payload)
	return f.startErr
}

func (f *fakeRemoteDesktopEngine) StopSession(sessionID string) error {
	f.stopCalls = append(f.stopCalls, sessionID)
	return f.stopErr
}

func (f *fakeRemoteDesktopEngine) UpdateSession(payload remotedesktop.RemoteDesktopCommandPayload) error {
	f.updateCalls = append(f.updateCalls, payload)
	return f.updateErr
}

func (f *fakeRemoteDesktopEngine) HandleInput(ctx context.Context, payload remotedesktop.RemoteDesktopCommandPayload) error {
	f.inputCalls = append(f.inputCalls, payload)
	return f.inputErr
}

func (f *fakeRemoteDesktopEngine) DeliverFrame(ctx context.Context, frame remotedesktop.RemoteDesktopFramePacket) error {
	f.deliverCalls = append(f.deliverCalls, frame)
	return f.deliverErr
}

func (f *fakeRemoteDesktopEngine) Shutdown() {
	f.shutdownCalled = true
}

func unwrapModuleResult(t *testing.T, err error) protocol.CommandResult {
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

func TestRemoteDesktopModuleInitUsesInjectedEngine(t *testing.T) {
	t.Parallel()

	engine := &fakeRemoteDesktopEngine{}
	module := newRemoteDesktopModule(engine)

	runtime := Config{AgentID: "agent-1", BaseURL: "https://controller.example"}
	if err := module.Init(context.Background(), runtime); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if len(engine.configureCalls) != 1 {
		t.Fatalf("expected configure to be called once, got %d", len(engine.configureCalls))
	}

	runtime.AuthKey = "key"
	if err := module.UpdateConfig(runtime); err != nil {
		t.Fatalf("UpdateConfig returned error: %v", err)
	}
	if len(engine.configureCalls) != 2 {
		t.Fatalf("expected configure to be called twice, got %d", len(engine.configureCalls))
	}
}

func TestRemoteDesktopModuleDelegatesCommandsToEngine(t *testing.T) {
	t.Parallel()

	engine := &fakeRemoteDesktopEngine{}
	module := newRemoteDesktopModule(engine)

	runtime := Config{AgentID: "agent-1"}
	if err := module.Init(context.Background(), runtime); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	ctx := context.Background()

	startPayload := remotedesktop.RemoteDesktopCommandPayload{Action: "start", SessionID: "session-1"}
	rawStart, err := json.Marshal(startPayload)
	if err != nil {
		t.Fatalf("marshal start payload: %v", err)
	}
	result := unwrapModuleResult(t, module.Handle(ctx, protocol.Command{ID: "start", Name: "remote-desktop", Payload: rawStart}))
	if !result.Success {
		t.Fatalf("expected start to succeed, got result: %+v", result)
	}
	if len(engine.startCalls) != 1 || engine.startCalls[0].SessionID != "session-1" {
		t.Fatalf("expected start payload to be forwarded, got %+v", engine.startCalls)
	}

	stopPayload := remotedesktop.RemoteDesktopCommandPayload{Action: "stop", SessionID: "session-1"}
	rawStop, err := json.Marshal(stopPayload)
	if err != nil {
		t.Fatalf("marshal stop payload: %v", err)
	}
	unwrapModuleResult(t, module.Handle(ctx, protocol.Command{ID: "stop", Name: "remote-desktop", Payload: rawStop}))
	if len(engine.stopCalls) != 1 || engine.stopCalls[0] != "session-1" {
		t.Fatalf("expected stop payload to be forwarded, got %+v", engine.stopCalls)
	}

	updatePayload := remotedesktop.RemoteDesktopCommandPayload{Action: "configure", SessionID: "session-1"}
	rawUpdate, err := json.Marshal(updatePayload)
	if err != nil {
		t.Fatalf("marshal configure payload: %v", err)
	}
	unwrapModuleResult(t, module.Handle(ctx, protocol.Command{ID: "configure", Name: "remote-desktop", Payload: rawUpdate}))
	if len(engine.updateCalls) != 1 {
		t.Fatalf("expected configure payload to be forwarded, got %d calls", len(engine.updateCalls))
	}

	inputPayload := remotedesktop.RemoteDesktopCommandPayload{
		Action:    "input",
		SessionID: "session-1",
		Events: []remotedesktop.RemoteDesktopInputEvent{{
			Type: remotedesktop.RemoteInputMouseMove,
		}},
	}
	rawInput, err := json.Marshal(inputPayload)
	if err != nil {
		t.Fatalf("marshal input payload: %v", err)
	}
	unwrapModuleResult(t, module.Handle(ctx, protocol.Command{ID: "input", Name: "remote-desktop", Payload: rawInput}))
	if len(engine.inputCalls) != 1 {
		t.Fatalf("expected input payload to be forwarded once, got %d calls", len(engine.inputCalls))
	}

	burstErr := module.HandleInputBurst(ctx, protocol.RemoteDesktopInputBurst{
		SessionID: "session-1",
		Events: []protocol.RemoteDesktopInputEvent{{
			Type:   protocol.RemoteDesktopInputType(remotedesktop.RemoteInputMouseMove),
			X:      1,
			Y:      2,
			Repeat: false,
		}},
	})
	if burstErr != nil {
		t.Fatalf("HandleInputBurst returned error: %v", burstErr)
	}
	if len(engine.inputCalls) != 2 {
		t.Fatalf("expected burst to forward input payload, got %d calls", len(engine.inputCalls))
	}

	if err := module.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
	if !engine.shutdownCalled {
		t.Fatal("expected Shutdown to invoke engine")
	}
}
