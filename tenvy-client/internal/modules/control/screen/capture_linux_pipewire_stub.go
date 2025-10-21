//go:build linux && !pipewire

package screen

import "errors"

func defaultPlatformCaptureCandidates() []backendCandidate {
	if err := ensurePipewireAvailable(); err != nil {
		return nil
	}
	return []backendCandidate{{
		name: "pipewire",
		factory: func() (captureBackend, error) {
			return nil, errors.New("PipeWire backend requires build tag 'pipewire'")
		},
	}}
}
