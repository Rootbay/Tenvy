//go:build linux

package screen

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
)

var (
	pipewireProbeOnce sync.Once
	pipewireProbeErr  error
)

func defaultPlatformCaptureCandidates() []backendCandidate {
	return []backendCandidate{{name: "pipewire", factory: newPipewireCaptureBackend}}
}

func newPipewireCaptureBackend() (captureBackend, error) {
	if err := ensurePipewireAvailable(); err != nil {
		return nil, err
	}
	return nil, errors.New("PipeWire capture backend not linked in this build")
}

func ensurePipewireAvailable() error {
	pipewireProbeOnce.Do(func() {
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if runtimeDir == "" {
			pipewireProbeErr = errors.New("XDG_RUNTIME_DIR not set")
			return
		}
		socketPath := filepath.Join(runtimeDir, "pipewire-0")
		if _, err := os.Stat(socketPath); err != nil {
			pipewireProbeErr = err
		}
	})
	return pipewireProbeErr
}
