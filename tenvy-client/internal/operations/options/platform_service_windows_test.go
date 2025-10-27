//go:build windows

package options

import (
	"context"
	"strings"
	"testing"
)

func TestWindowsVisualDistortionInvertColors(t *testing.T) {
	service := &windowsPlatformService{}
	original := configureColorFilterFunc
	called := false
	configureColorFilterFunc = func(ctx context.Context, active bool, filterType int) error {
		called = true
		if !active || filterType != 1 {
			t.Fatalf("unexpected arguments active=%v filterType=%d", active, filterType)
		}
		return nil
	}
	t.Cleanup(func() { configureColorFilterFunc = original })

	summary, err := service.Execute(context.Background(), "visual-distortion", map[string]any{"mode": "InvertColors"}, State{})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if summary != "Enabled Windows color inversion filter" {
		t.Fatalf("unexpected summary: %q", summary)
	}
	if !called {
		t.Fatalf("expected ConfigureColorFilter to be invoked")
	}
}

func TestWindowsVisualDistortionUnsupported(t *testing.T) {
	service := &windowsPlatformService{}
	original := configureColorFilterFunc
	configureColorFilterFunc = func(ctx context.Context, active bool, filterType int) error {
		t.Fatalf("ConfigureColorFilter should not be called for unsupported mode")
		return nil
	}
	t.Cleanup(func() { configureColorFilterFunc = original })

	summary, err := service.Execute(context.Background(), "visual-distortion", map[string]any{"mode": "Pixelate"}, State{})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(summary, "unsupported") {
		t.Fatalf("expected unsupported summary, got %q", summary)
	}
}

func TestWindowsCursorBehaviors(t *testing.T) {
	service := &windowsPlatformService{}
	original := configureCursorStateFunc
	var lastArgs struct {
		swap   bool
		trails int
	}
	configureCursorStateFunc = func(ctx context.Context, swap bool, trails int) error {
		lastArgs.swap = swap
		lastArgs.trails = trails
		return nil
	}
	t.Cleanup(func() { configureCursorStateFunc = original })

	summary, err := service.Execute(context.Background(), "cursor-behavior", map[string]any{"behavior": "Reverse"}, State{})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(summary, "Swapped") {
		t.Fatalf("unexpected summary: %q", summary)
	}
	if !lastArgs.swap || lastArgs.trails != 0 {
		t.Fatalf("unexpected cursor configuration: %+v", lastArgs)
	}

	summary, err = service.Execute(context.Background(), "cursor-behavior", map[string]any{"behavior": "Ghost"}, State{})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(summary, "cursor trails") {
		t.Fatalf("unexpected summary: %q", summary)
	}
	if lastArgs.swap || lastArgs.trails != 7 {
		t.Fatalf("unexpected cursor configuration: %+v", lastArgs)
	}

	summary, err = service.Execute(context.Background(), "cursor-behavior", map[string]any{"behavior": "Unknown"}, State{})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(summary, "unsupported") {
		t.Fatalf("expected unsupported summary, got %q", summary)
	}
}

func TestWindowsFakeEventFallback(t *testing.T) {
	service := &windowsPlatformService{}
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
