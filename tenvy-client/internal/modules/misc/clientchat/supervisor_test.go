package clientchat

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

func TestSupervisorRespawnsSessionOnCrash(t *testing.T) {
	supervisor := NewSupervisor(Config{})
	sessionID, created := supervisor.ensureSession("")
	if sessionID == "" || !created {
		t.Fatalf("expected supervisor to create session, got id=%q created=%v", sessionID, created)
	}

	supervisor.mu.Lock()
	original := supervisor.session
	supervisor.mu.Unlock()
	if original == nil {
		t.Fatal("expected session to be initialized")
	}

	original.terminate(reasonCrash)

	deadline := time.Now().Add(200 * time.Millisecond)
	for {
		supervisor.mu.Lock()
		replacement := supervisor.session
		supervisor.mu.Unlock()
		if replacement != nil && replacement != original {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("supervisor did not respawn session after crash")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSupervisorStopSessionPreventsRespawn(t *testing.T) {
	supervisor := NewSupervisor(Config{})
	sessionID, _ := supervisor.ensureSession("")
	supervisor.mu.Lock()
	current := supervisor.session
	supervisor.mu.Unlock()
	if current == nil {
		t.Fatal("expected session to be initialized")
	}

	if err := supervisor.stopSession(sessionID); err != nil {
		t.Fatalf("stopSession error: %v", err)
	}

	current.terminate(reasonCrash)
	time.Sleep(20 * time.Millisecond)
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	if supervisor.session != nil {
		t.Fatal("expected supervisor session to remain nil after stop")
	}
}

func TestSupervisorHandleCommandLifecycle(t *testing.T) {
	supervisor := NewSupervisor(Config{})
	payload, err := json.Marshal(protocol.ClientChatCommandPayload{Action: "start"})
	if err != nil {
		t.Fatalf("marshal start payload: %v", err)
	}
	startResult := supervisor.HandleCommand(context.Background(), protocol.Command{ID: "start", Payload: payload})
	if !startResult.Success {
		t.Fatalf("start command failed: %v", startResult.Error)
	}

	supervisor.mu.Lock()
	active := supervisor.session
	supervisor.mu.Unlock()
	if active == nil {
		t.Fatal("expected active session after start")
	}

	stopPayload, err := json.Marshal(protocol.ClientChatCommandPayload{Action: "stop", SessionID: active.id})
	if err != nil {
		t.Fatalf("marshal stop payload: %v", err)
	}

	stopResult := supervisor.HandleCommand(context.Background(), protocol.Command{ID: "stop", Payload: stopPayload})
	if !stopResult.Success {
		t.Fatalf("stop command failed: %v", stopResult.Error)
	}

	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	if supervisor.session != nil {
		t.Fatal("expected session to be cleared after stop command")
	}
}

func TestSupervisorIgnoresUnstoppableDisable(t *testing.T) {
	supervisor := NewSupervisor(Config{})
	if _, created := supervisor.ensureSession(""); !created {
		t.Fatal("expected session creation")
	}

	falseValue := false
	payload, err := json.Marshal(protocol.ClientChatCommandPayload{
		Action: "configure",
		Features: &protocol.ClientChatFeatureFlags{
			Unstoppable: &falseValue,
		},
	})
	if err != nil {
		t.Fatalf("marshal configure payload: %v", err)
	}

	result := supervisor.HandleCommand(context.Background(), protocol.Command{ID: "configure", Payload: payload})
	if !result.Success {
		t.Fatalf("configure command failed: %v", result.Error)
	}

	supervisor.mu.Lock()
	unstoppable := supervisor.unstoppable
	supervisor.mu.Unlock()
	if !unstoppable {
		t.Fatal("expected unstoppable flag to remain true")
	}
}
