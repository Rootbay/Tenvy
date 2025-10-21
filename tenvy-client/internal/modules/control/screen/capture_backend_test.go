package screen

import (
	"errors"
	"image"
	"testing"
)

type testBackend struct {
	name      string
	captureFn func(image.Rectangle) (*image.RGBA, error)
}

func (t testBackend) Capture(bounds image.Rectangle) (*image.RGBA, error) {
	if t.captureFn != nil {
		return t.captureFn(bounds)
	}
	return image.NewRGBA(bounds), nil
}

func (t testBackend) Name() string { return t.name }

func TestSafeCaptureRect_UsesPreferredBackend(t *testing.T) {
	resetCaptureBackendForTesting()
	oldCandidates := platformCaptureCandidates
	platformCaptureCandidates = func() []backendCandidate {
		return []backendCandidate{{
			name: "test",
			factory: func() (captureBackend, error) {
				return testBackend{name: "test"}, nil
			},
		}}
	}
	defer func() {
		platformCaptureCandidates = oldCandidates
		resetCaptureBackendForTesting()
	}()

	fallbackCaptureFactory = func() (captureBackend, error) {
		t.Fatal("fallback backend should not be used when preferred backend succeeds")
		return nil, nil
	}

	bounds := image.Rect(0, 0, 4, 4)
	img, err := SafeCaptureRect(bounds)
	if err != nil {
		t.Fatalf("SafeCaptureRect returned error: %v", err)
	}
	if img == nil || img.Bounds() != bounds {
		t.Fatalf("unexpected capture image: %#v", img)
	}
	if got := SelectedBackend(); got != "test" {
		t.Fatalf("expected selected backend to be 'test', got %q", got)
	}
	if errs := CapabilityErrors(); len(errs) != 0 {
		t.Fatalf("expected no capability errors, got %d", len(errs))
	}
}

func TestSafeCaptureRect_FallsBackToScreenshot(t *testing.T) {
	resetCaptureBackendForTesting()
	oldCandidates := platformCaptureCandidates
	platformCaptureCandidates = func() []backendCandidate {
		return []backendCandidate{{
			name: "pipewire",
			factory: func() (captureBackend, error) {
				return nil, errors.New("pipewire unavailable")
			},
		}}
	}
	defer func() {
		platformCaptureCandidates = oldCandidates
		resetCaptureBackendForTesting()
	}()

	fallbackCaptureFactory = func() (captureBackend, error) {
		return testBackend{name: "fallback"}, nil
	}

	bounds := image.Rect(0, 0, 2, 2)
	img, err := SafeCaptureRect(bounds)
	if err != nil {
		t.Fatalf("SafeCaptureRect returned error: %v", err)
	}
	if img == nil {
		t.Fatal("expected fallback backend to provide an image")
	}
	if got := SelectedBackend(); got != "fallback" {
		t.Fatalf("expected fallback backend to be selected, got %q", got)
	}
	errs := CapabilityErrors()
	if len(errs) != 1 {
		t.Fatalf("expected 1 capability error, got %d", len(errs))
	}
	if errs[0].Backend != "pipewire" {
		t.Fatalf("expected capability error for pipewire, got %q", errs[0].Backend)
	}
}
