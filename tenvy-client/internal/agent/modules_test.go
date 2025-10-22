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
	metadata       ModuleMetadata
	initErr        error
	updateErr      error
	handleFunc     func(context.Context, protocol.Command) protocol.CommandResult
	initCalled     int
	updateCalled   int
	shutdownCalled int
	handleCalled   int
	log            *[]string
}

func (m *stubModule) Metadata() ModuleMetadata {
	return m.metadata
}

func (m *stubModule) Init(context.Context, ModuleRuntime) error {
	m.initCalled++
	if m.log != nil {
		*m.log = append(*m.log, m.metadata.ID+":init")
	}
	return m.initErr
}

func (m *stubModule) Handle(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
	m.handleCalled++
	if m.log != nil {
		*m.log = append(*m.log, m.metadata.ID+":handle"+":"+cmd.Name)
	}
	if m.handleFunc != nil {
		return m.handleFunc(ctx, cmd)
	}
	return protocol.CommandResult{CommandID: cmd.ID, Success: true, CompletedAt: "now"}
}

func (m *stubModule) UpdateConfig(context.Context, ModuleRuntime) error {
	m.updateCalled++
	if m.log != nil {
		*m.log = append(*m.log, m.metadata.ID+":update")
	}
	return m.updateErr
}

func (m *stubModule) Shutdown(context.Context) {
	m.shutdownCalled++
	if m.log != nil {
		*m.log = append(*m.log, m.metadata.ID+":shutdown")
	}
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
		handleFunc: func(_ context.Context, cmd protocol.Command) protocol.CommandResult {
			return protocol.CommandResult{CommandID: cmd.ID, Success: true, Output: "ok", CompletedAt: "done"}
		},
		log: &callLog,
	}

	manager := newModuleManager()
	manager.register(moduleA)
	manager.register(moduleB)

	runtime := ModuleRuntime{AgentID: "agent-123"}
	if err := manager.Init(context.Background(), runtime); err != nil {
		t.Fatalf("Init returned unexpected error: %v", err)
	}

	if moduleA.initCalled != 1 || moduleB.initCalled != 1 {
		t.Fatalf("expected init to be called once on each module, got %d and %d", moduleA.initCalled, moduleB.initCalled)
	}

	err := manager.UpdateConfig(context.Background(), runtime)
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

	manager.Shutdown(context.Background())

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

func TestRemoteDesktopModuleInitUsesInjectedEngine(t *testing.T) {
	t.Parallel()

	engine := &fakeRemoteDesktopEngine{}
	module := newRemoteDesktopModule(engine)

	runtime := ModuleRuntime{AgentID: "agent-1", BaseURL: "https://controller.example"}
	if err := module.Init(context.Background(), runtime); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if len(engine.configureCalls) != 1 {
		t.Fatalf("expected configure to be called once, got %d", len(engine.configureCalls))
	}

	runtime.AuthKey = "key"
	if err := module.UpdateConfig(context.Background(), runtime); err != nil {
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

	runtime := ModuleRuntime{AgentID: "agent-1"}
	if err := module.Init(context.Background(), runtime); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	ctx := context.Background()

	startPayload := remotedesktop.RemoteDesktopCommandPayload{Action: "start", SessionID: "session-1"}
	rawStart, err := json.Marshal(startPayload)
	if err != nil {
		t.Fatalf("marshal start payload: %v", err)
	}
	result := module.Handle(ctx, protocol.Command{ID: "start", Name: "remote-desktop", Payload: rawStart})
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
	_ = module.Handle(ctx, protocol.Command{ID: "stop", Name: "remote-desktop", Payload: rawStop})
	if len(engine.stopCalls) != 1 || engine.stopCalls[0] != "session-1" {
		t.Fatalf("expected stop payload to be forwarded, got %+v", engine.stopCalls)
	}

	updatePayload := remotedesktop.RemoteDesktopCommandPayload{Action: "configure", SessionID: "session-1"}
	rawUpdate, err := json.Marshal(updatePayload)
	if err != nil {
		t.Fatalf("marshal configure payload: %v", err)
	}
	_ = module.Handle(ctx, protocol.Command{ID: "configure", Name: "remote-desktop", Payload: rawUpdate})
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
	_ = module.Handle(ctx, protocol.Command{ID: "input", Name: "remote-desktop", Payload: rawInput})
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

	module.Shutdown(ctx)
	if !engine.shutdownCalled {
		t.Fatal("expected Shutdown to invoke engine")
	}
}
