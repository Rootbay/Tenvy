//go:build linux

package remotedesktop

import (
	"errors"
	"strings"
	"testing"
)

type stubBackend struct {
	called bool
	err    error
}

func (s *stubBackend) Process(_ []remoteMonitor, _ RemoteDesktopSettings, events []RemoteDesktopInputEvent) error {
	if len(events) == 0 {
		return nil
	}
	s.called = true
	return s.err
}

func TestProcessRemoteInput_UsesWaylandWhenAvailable(t *testing.T) {
	oldWaylandFactory := waylandBackendFactory
	oldX11Factory := x11BackendFactory
	defer func() {
		waylandBackendFactory = oldWaylandFactory
		x11BackendFactory = oldX11Factory
	}()

	stubWayland := &stubBackend{}
	waylandBackendFactory = func() (linuxInputBackend, error) { return stubWayland, nil }
	x11BackendFactory = func() (linuxInputBackend, error) {
		t.Fatalf("x11 backend should not be invoked when Wayland succeeds")
		return nil, nil
	}

	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("DISPLAY", "")

	events := []RemoteDesktopInputEvent{{Type: RemoteInputMouseMove, X: 0.5, Y: 0.5, Normalized: true}}
	if err := processRemoteInput(nil, RemoteDesktopSettings{}, events); err != nil {
		t.Fatalf("processRemoteInput returned error: %v", err)
	}
	if !stubWayland.called {
		t.Fatalf("expected Wayland backend to handle the events")
	}
}

func TestProcessRemoteInput_FallsBackToX11(t *testing.T) {
	oldWaylandFactory := waylandBackendFactory
	oldX11Factory := x11BackendFactory
	defer func() {
		waylandBackendFactory = oldWaylandFactory
		x11BackendFactory = oldX11Factory
	}()

	sentinel := errors.New("wayland backend failed")
	waylandBackendFactory = func() (linuxInputBackend, error) { return nil, sentinel }
	stubX11 := &stubBackend{}
	x11BackendFactory = func() (linuxInputBackend, error) { return stubX11, nil }

	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("DISPLAY", ":0")

	events := []RemoteDesktopInputEvent{{Type: RemoteInputMouseButton, Button: RemoteMouseButtonLeft, Pressed: true}}
	if err := processRemoteInput(nil, RemoteDesktopSettings{}, events); err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}
	if !stubX11.called {
		t.Fatalf("expected X11 backend to process events after Wayland failure")
	}
}

func TestProcessRemoteInput_WaylandErrorWithoutX11(t *testing.T) {
	oldWaylandFactory := waylandBackendFactory
	oldX11Factory := x11BackendFactory
	defer func() {
		waylandBackendFactory = oldWaylandFactory
		x11BackendFactory = oldX11Factory
	}()

	waylandBackendFactory = func() (linuxInputBackend, error) {
		return nil, errors.New("permission denied opening /dev/uinput")
	}
	x11BackendFactory = func() (linuxInputBackend, error) {
		t.Fatalf("x11 backend should not be invoked without DISPLAY")
		return nil, nil
	}

	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("DISPLAY", "")

	events := []RemoteDesktopInputEvent{{Type: RemoteInputKey, Key: "a", Pressed: true}}
	err := processRemoteInput(nil, RemoteDesktopSettings{}, events)
	if err == nil {
		t.Fatalf("expected an error when Wayland backend is unavailable")
	}
	if strings.Contains(err.Error(), "DISPLAY") {
		t.Fatalf("unexpected DISPLAY-related error: %v", err)
	}
}
