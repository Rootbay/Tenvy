package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

func TestAgentControlCommandHandlerDisconnect(t *testing.T) {
	var agent Agent
	cmd := protocol.Command{
		ID:      "cmd-disconnect",
		Name:    "agent-control",
		Payload: mustMarshalAgentControlPayload(t, protocol.AgentControlCommandPayload{Action: "disconnect"}),
	}

	result := agentControlCommandHandler(context.Background(), &agent, cmd)
	if !result.Success {
		t.Fatalf("expected success result, got %+v", result)
	}

	if directive := agent.connectionFlag.Load(); directive != connectionDirectiveDisconnect {
		t.Fatalf("expected disconnect directive, got %d", directive)
	}
}

func TestAgentControlCommandHandlerReconnect(t *testing.T) {
	var agent Agent
	cmd := protocol.Command{
		ID:      "cmd-reconnect",
		Name:    "agent-control",
		Payload: mustMarshalAgentControlPayload(t, protocol.AgentControlCommandPayload{Action: "reconnect"}),
	}

	result := agentControlCommandHandler(context.Background(), &agent, cmd)
	if !result.Success {
		t.Fatalf("expected success result, got %+v", result)
	}

	if directive := agent.connectionFlag.Load(); directive != connectionDirectiveReconnect {
		t.Fatalf("expected reconnect directive, got %d", directive)
	}
}

func TestAgentControlCommandHandlerReconnectDoesNotOverrideDisconnect(t *testing.T) {
	var agent Agent

	disconnectCmd := protocol.Command{
		ID:      "cmd-disconnect",
		Name:    "agent-control",
		Payload: mustMarshalAgentControlPayload(t, protocol.AgentControlCommandPayload{Action: "disconnect"}),
	}
	reconnectCmd := protocol.Command{
		ID:      "cmd-reconnect",
		Name:    "agent-control",
		Payload: mustMarshalAgentControlPayload(t, protocol.AgentControlCommandPayload{Action: "reconnect"}),
	}

	if res := agentControlCommandHandler(context.Background(), &agent, disconnectCmd); !res.Success {
		t.Fatalf("disconnect command failed: %+v", res)
	}
	if res := agentControlCommandHandler(context.Background(), &agent, reconnectCmd); !res.Success {
		t.Fatalf("reconnect command failed: %+v", res)
	}

	if directive := agent.connectionFlag.Load(); directive != connectionDirectiveDisconnect {
		t.Fatalf("expected disconnect directive to persist, got %d", directive)
	}
}

func TestAgentControlCommandHandlerPowerActions(t *testing.T) {
	restore := stubPowerFunctions()
	defer restore()

	tests := []struct {
		name     string
		action   string
		stub     func() error
		expected string
	}{
		{
			name:   "shutdown",
			action: "shutdown",
			stub: func() error {
				return nil
			},
			expected: "shutdown requested",
		},
		{
			name:   "restart",
			action: "restart",
			stub: func() error {
				return nil
			},
			expected: "restart requested",
		},
		{
			name:   "sleep",
			action: "sleep",
			stub: func() error {
				return nil
			},
			expected: "sleep requested",
		},
		{
			name:   "logoff",
			action: "logoff",
			stub: func() error {
				return nil
			},
			expected: "logoff requested",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			called := false
			assignPowerFunc(tt.action, func() error {
				called = true
				return tt.stub()
			})

			cmd := protocol.Command{
				ID:      "cmd-" + tt.action,
				Name:    "agent-control",
				Payload: mustMarshalAgentControlPayload(t, protocol.AgentControlCommandPayload{Action: tt.action}),
			}

			res := agentControlCommandHandler(context.Background(), &Agent{}, cmd)
			if !res.Success {
				t.Fatalf("expected success for %s, got %+v", tt.action, res)
			}
			if res.Output != tt.expected {
				t.Fatalf("expected output %q, got %q", tt.expected, res.Output)
			}
			if !called {
				t.Fatalf("expected %s helper to be invoked", tt.action)
			}
		})
	}
}

func TestAgentControlCommandHandlerPowerActionFailure(t *testing.T) {
	restore := stubPowerFunctions()
	defer restore()

	expectedErr := errors.New("boom")
	assignPowerFunc("shutdown", func() error {
		return expectedErr
	})

	cmd := protocol.Command{
		ID:      "cmd-shutdown-failure",
		Name:    "agent-control",
		Payload: mustMarshalAgentControlPayload(t, protocol.AgentControlCommandPayload{Action: "shutdown"}),
	}

	res := agentControlCommandHandler(context.Background(), &Agent{}, cmd)
	if res.Success {
		t.Fatalf("expected failure, got %+v", res)
	}
	if res.Error == "" {
		t.Fatalf("expected failure error message, got empty string")
	}
}

func stubPowerFunctions() func() {
	shutdownOriginal := agentShutdownFunc
	restartOriginal := agentRestartFunc
	sleepOriginal := agentSleepFunc
	logoffOriginal := agentLogoffFunc

	return func() {
		agentShutdownFunc = shutdownOriginal
		agentRestartFunc = restartOriginal
		agentSleepFunc = sleepOriginal
		agentLogoffFunc = logoffOriginal
	}
}

func assignPowerFunc(action string, fn func() error) {
	switch action {
	case "shutdown":
		agentShutdownFunc = fn
	case "restart":
		agentRestartFunc = fn
	case "sleep":
		agentSleepFunc = fn
	case "logoff":
		agentLogoffFunc = fn
	default:
		panic("unknown action: " + action)
	}
}

func mustMarshalAgentControlPayload(t *testing.T, payload protocol.AgentControlCommandPayload) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return data
}
