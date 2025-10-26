package geolocation

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
		c.now = time.Date(2024, 6, 1, 8, 0, 0, 0, time.UTC)
	}
	return c.now
}

func TestManagerStatus(t *testing.T) {
	manager := NewManager()
	manager.setClock(&stubClock{})

	result := manager.HandleCommand(context.Background(), protocol.Command{ID: "status"})
	if !result.Success {
		t.Fatalf("expected success, got %s", result.Error)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(result.Output), &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if decoded["action"] != "status" {
		t.Fatalf("expected status action, got %v", decoded["action"])
	}
}

func TestManagerLookup(t *testing.T) {
	manager := NewManager()
	manager.setClock(&stubClock{})

	payload, err := json.Marshal(commandPayload{Action: "lookup", IP: "203.0.113.10", Provider: "maxmind", IncludeTimezone: true, IncludeMap: true})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result := manager.HandleCommand(context.Background(), protocol.Command{ID: "lookup", Payload: payload})
	if !result.Success {
		t.Fatalf("expected success, got %s", result.Error)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(result.Output), &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if decoded["action"] != "lookup" {
		t.Fatalf("expected lookup action, got %v", decoded["action"])
	}
}
