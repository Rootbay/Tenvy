//go:build linux

package screen

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPipewireCandidatesAdvertisedOnlyWhenReachable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pipewire detection only runs on linux")
	}

	origRuntime := os.Getenv("XDG_RUNTIME_DIR")
	t.Cleanup(func() {
		os.Setenv("XDG_RUNTIME_DIR", origRuntime)
		resetPipewireProbeForTesting()
	})

	os.Unsetenv("XDG_RUNTIME_DIR")
	resetPipewireProbeForTesting()
	if candidates := defaultPlatformCaptureCandidates(); len(candidates) != 0 {
		t.Fatalf("expected no pipewire candidates when runtime dir unset, got %d", len(candidates))
	}

	dir := t.TempDir()
	os.Setenv("XDG_RUNTIME_DIR", dir)

	socketPath := filepath.Join(dir, "pipewire-0")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create test pipewire socket: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	resetPipewireProbeForTesting()
	candidates := defaultPlatformCaptureCandidates()
	if len(candidates) != 1 {
		t.Fatalf("expected pipewire candidate when socket reachable, got %d", len(candidates))
	}
	if candidates[0].name != "pipewire" {
		t.Fatalf("unexpected candidate name %q", candidates[0].name)
	}
}
