package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

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
