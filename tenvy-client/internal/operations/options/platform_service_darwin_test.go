//go:build darwin

package options

import (
	"context"
	"strings"
	"testing"
)

func TestDarwinUnsupportedDisplayOptions(t *testing.T) {
	service := newPlatformService()
	summary, err := service.Execute(context.Background(), "visual-distortion", map[string]any{"mode": "Wiggle"}, State{})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(summary, "unsupported") {
		t.Fatalf("expected unsupported summary, got %q", summary)
	}

	summary, err = service.Execute(context.Background(), "cursor-behavior", map[string]any{"behavior": "Ghost"}, State{})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(summary, "unsupported") {
		t.Fatalf("expected unsupported summary, got %q", summary)
	}
}

func TestDarwinFakeEventFallback(t *testing.T) {
	service := newPlatformService()
	summary, err := service.Execute(context.Background(), "fake-event-mode", map[string]any{"mode": "None"}, State{})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(strings.ToLower(summary), "cleared") {
		t.Fatalf("expected cleared summary, got %q", summary)
	}

	summary, err = service.Execute(context.Background(), "fake-event-mode", map[string]any{"mode": "FakeUpdate"}, State{})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(summary, "unsupported") {
		t.Fatalf("expected unsupported summary, got %q", summary)
	}
}
