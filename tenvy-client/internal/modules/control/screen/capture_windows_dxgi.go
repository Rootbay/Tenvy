//go:build windows

package screen

import (
	"errors"
	"sync"

	"golang.org/x/sys/windows"
)

var (
	dxgiProbeOnce sync.Once
	dxgiProbeErr  error
)

func defaultPlatformCaptureCandidates() []backendCandidate {
	return []backendCandidate{{name: "dxgi", factory: newDXGICaptureBackend}}
}

func newDXGICaptureBackend() (captureBackend, error) {
	if err := ensureDXGICapable(); err != nil {
		return nil, err
	}
	return nil, errors.New("dxgi desktop duplication backend not linked in this build")
}

func ensureDXGICapable() error {
	dxgiProbeOnce.Do(func() {
		dxgi := windows.NewLazySystemDLL("dxgi.dll")
		if err := dxgi.Load(); err != nil {
			dxgiProbeErr = err
			return
		}
		d3d11 := windows.NewLazySystemDLL("d3d11.dll")
		if err := d3d11.Load(); err != nil {
			dxgiProbeErr = err
			return
		}
	})
	return dxgiProbeErr
}
