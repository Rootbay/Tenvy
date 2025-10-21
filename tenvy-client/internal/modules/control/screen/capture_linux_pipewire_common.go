//go:build linux

package screen

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	pipewireProbeOnce sync.Once
	pipewireProbeErr  error
)

func defaultPipewireSocketPath() (string, error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return "", fmt.Errorf("XDG_RUNTIME_DIR not set")
	}
	return filepath.Join(runtimeDir, "pipewire-0"), nil
}

func ensurePipewireAvailable() error {
	pipewireProbeOnce.Do(func() {
		socketPath, err := defaultPipewireSocketPath()
		if err != nil {
			pipewireProbeErr = err
			return
		}
		info, err := os.Stat(socketPath)
		if err != nil {
			pipewireProbeErr = fmt.Errorf("pipewire socket: %w", err)
			return
		}
		if info.Mode()&os.ModeSocket == 0 {
			pipewireProbeErr = fmt.Errorf("pipewire socket: %s is not a socket", socketPath)
			return
		}
		pipewireProbeErr = nil
	})
	return pipewireProbeErr
}

func resetPipewireProbeForTesting() {
	pipewireProbeOnce = sync.Once{}
	pipewireProbeErr = nil
}
