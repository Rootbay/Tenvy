package agent

import (
	"context"
	"testing"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

func TestCommandRouterDispatchesBuiltins(t *testing.T) {
	router, err := newDefaultCommandRouter()
	if err != nil {
		t.Fatalf("newDefaultCommandRouter() error = %v", err)
	}

	agent := &Agent{commands: router}
	cmd := protocol.Command{ID: "cmd-1", Name: "ping"}

	result := router.dispatch(context.Background(), agent, cmd)
	if !result.Success {
		t.Fatalf("expected ping command to succeed, got result: %+v", result)
	}
	if result.Output != "pong" {
		t.Fatalf("unexpected ping output: %q", result.Output)
	}
}

func TestCommandRouterTrimsNameDuringDispatch(t *testing.T) {
	router, err := newDefaultCommandRouter()
	if err != nil {
		t.Fatalf("newDefaultCommandRouter() error = %v", err)
	}

	cmd := protocol.Command{ID: "cmd-2", Name: "  shell  ", Payload: []byte(`{"command":"echo hi"}`)}
	agent := &Agent{commands: router}

	result := router.dispatch(context.Background(), agent, cmd)
	if !result.Success {
		t.Fatalf("expected trimmed shell command to succeed, got result: %+v", result)
	}
}

func TestCommandRouterRejectsDuplicateRegistration(t *testing.T) {
	router := newCommandRouter()
	if err := router.register("ping", pingCommandHandler); err != nil {
		t.Fatalf("unexpected error registering ping: %v", err)
	}
	if err := router.register("ping", pingCommandHandler); err == nil {
		t.Fatalf("expected duplicate registration error")
	}
}

func TestCommandRouterUnsupportedCommand(t *testing.T) {
	router, err := newDefaultCommandRouter()
	if err != nil {
		t.Fatalf("newDefaultCommandRouter() error = %v", err)
	}

	cmd := protocol.Command{ID: "cmd-3", Name: "does-not-exist"}
	result := router.dispatch(context.Background(), &Agent{commands: router}, cmd)
	if result.Success {
		t.Fatalf("expected unsupported command to fail, got %+v", result)
	}
	if result.Error == "" {
		t.Fatalf("expected unsupported command to include error message")
	}
}
