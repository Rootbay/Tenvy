package environment

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

type stubClock struct {
	now time.Time
}

func (c *stubClock) Now() time.Time {
	if c.now.IsZero() {
		c.now = time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	}
	return c.now
}

func TestManagerHandleList(t *testing.T) {
	t.Setenv("TEST_ENV_ALPHA", "one")
	t.Setenv("TEST_ENV_BRAVO", "two")

	manager := NewManager()
	manager.setClock(&stubClock{})

	result := manager.HandleCommand(context.Background(), protocol.Command{ID: "list"})
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Output == "" {
		t.Fatalf("expected output payload")
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(result.Output), &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if decoded["action"] != "list" {
		t.Fatalf("expected action list, got %v", decoded["action"])
	}
}

func TestManagerHandleSet(t *testing.T) {
	manager := NewManager()
	manager.setClock(&stubClock{})

	payload, err := json.Marshal(commandPayload{Action: "set", Key: "TEST_ENV_SET", Value: "value", Scope: "machine"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result := manager.HandleCommand(context.Background(), protocol.Command{ID: "set", Payload: payload})
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if got, ok := os.LookupEnv("TEST_ENV_SET"); !ok || got != "value" {
		t.Fatalf("expected environment variable to be set, got %q", got)
	}
}

func TestManagerHandleRemove(t *testing.T) {
	t.Setenv("TEST_ENV_REMOVE", "payload")

	manager := NewManager()
	manager.setClock(&stubClock{})

	payload, err := json.Marshal(commandPayload{Action: "remove", Key: "TEST_ENV_REMOVE"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result := manager.HandleCommand(context.Background(), protocol.Command{ID: "remove", Payload: payload})
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if _, ok := os.LookupEnv("TEST_ENV_REMOVE"); ok {
		t.Fatalf("expected environment variable to be removed")
	}
}
