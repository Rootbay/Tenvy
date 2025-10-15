package agent

import (
	"context"
	"encoding/json"
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

func mustMarshalAgentControlPayload(t *testing.T, payload protocol.AgentControlCommandPayload) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return data
}
