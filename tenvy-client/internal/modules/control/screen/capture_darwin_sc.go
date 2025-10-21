//go:build darwin

package screen

import "errors"

func defaultPlatformCaptureCandidates() []backendCandidate {
	return []backendCandidate{{name: "screencapturekit", factory: newScreenCaptureKitBackend}}
}

func newScreenCaptureKitBackend() (captureBackend, error) {
	return nil, errors.New("ScreenCaptureKit backend not linked in this build")
}
