package trigger

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

type stubClock struct {
	now time.Time
}

func (c *stubClock) Now() time.Time {
	if c.now.IsZero() {
		c.now = time.Date(2024, 6, 1, 9, 30, 0, 0, time.UTC)
	}
	return c.now
}

func TestManagerStatus(t *testing.T) {
	manager := NewManager()
	manager.setClock(&stubClock{})

	result := manager.HandleCommand(context.Background(), protocol.Command{ID: "status"})
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
	if decoded["action"] != "status" {
		t.Fatalf("expected action status, got %v", decoded["action"])
	}
}

func TestManagerConfigure(t *testing.T) {
	manager := NewManager()
	manager.setClock(&stubClock{})

	payload, err := json.Marshal(commandPayload{
		Action: "configure",
		Config: monitorCommand{Feed: "batch", RefreshSeconds: 10, IncludeScreenshots: true, IncludeCommands: false},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result := manager.HandleCommand(context.Background(), protocol.Command{ID: "configure", Payload: payload})
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(result.Output), &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if decoded["action"] != "configure" {
		t.Fatalf("expected configure action, got %v", decoded["action"])
	}
}
