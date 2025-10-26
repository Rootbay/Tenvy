package options

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type serviceCall struct {
	operation string
	metadata  map[string]any
	state     State
}

type fakePlatformService struct {
	mu    sync.Mutex
	calls []serviceCall
	fn    func(ctx context.Context, operation string, metadata map[string]any, state State) (string, error)
}

func (f *fakePlatformService) Execute(
	ctx context.Context,
	operation string,
	metadata map[string]any,
	state State,
) (string, error) {
	f.mu.Lock()
	var copied map[string]any
	if len(metadata) > 0 {
		copied = make(map[string]any, len(metadata))
		for key, value := range metadata {
			copied[key] = value
		}
	} else {
		copied = make(map[string]any)
	}
	f.calls = append(f.calls, serviceCall{operation: operation, metadata: copied, state: state})
	handler := f.fn
	f.mu.Unlock()
	if handler != nil {
		return handler(ctx, operation, copied, state)
	}
	return "", nil
}

func (f *fakePlatformService) lastCall() serviceCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return serviceCall{}
	}
	return f.calls[len(f.calls)-1]
}

func TestApplyOperationUsesPlatformServiceSummary(t *testing.T) {
	mgr := NewManager(ManagerOptions{})
	fake := &fakePlatformService{}
	fake.fn = func(ctx context.Context, operation string, metadata map[string]any, state State) (string, error) {
		if operation != "defender-exclusion" {
			t.Fatalf("unexpected operation: %s", operation)
		}
		if enabled, ok := metadata["enabled"].(bool); !ok || !enabled {
			t.Fatalf("expected metadata enabled=true, got %v", metadata)
		}
		if state.DefenderExclusion {
			t.Fatalf("expected previous state to be false")
		}
		return "defender platform success", nil
	}
	mgr.SetPlatformService(fake)

	summary, err := mgr.ApplyOperation(context.Background(), "defender-exclusion", map[string]any{"enabled": true}, nil)
	if err != nil {
		t.Fatalf("ApplyOperation returned error: %v", err)
	}
	if summary != "defender platform success" {
		t.Fatalf("expected platform summary, got %q", summary)
	}

	snapshot := mgr.Snapshot()
	if !snapshot.DefenderExclusion {
		t.Fatalf("expected defender exclusion to be enabled")
	}

	if len(fake.calls) != 1 {
		t.Fatalf("expected 1 platform invocation, got %d", len(fake.calls))
	}
}

func TestApplyOperationErrorPreventsStateMutation(t *testing.T) {
	mgr := NewManager(ManagerOptions{})
	fake := &fakePlatformService{}
	fake.fn = func(ctx context.Context, operation string, metadata map[string]any, state State) (string, error) {
		return "", errors.New("platform failure")
	}
	mgr.SetPlatformService(fake)

	_, err := mgr.ApplyOperation(context.Background(), "defender-exclusion", map[string]any{"enabled": true}, nil)
	if err == nil {
		t.Fatalf("expected error from platform service")
	}
	if snapshot := mgr.Snapshot(); snapshot.DefenderExclusion {
		t.Fatalf("expected defender exclusion to remain disabled after failure")
	}
}

func TestSoundVolumeClampsAndInvokesService(t *testing.T) {
	mgr := NewManager(ManagerOptions{})
	fake := &fakePlatformService{}
	fake.fn = func(ctx context.Context, operation string, metadata map[string]any, state State) (string, error) {
		if operation != "sound-volume" {
			return "", nil
		}
		if volume, ok := metadata["volume"].(int); !ok || volume != 100 {
			t.Fatalf("expected volume metadata to be 100, got %v", metadata)
		}
		return "volume applied", nil
	}
	mgr.SetPlatformService(fake)

	summary, err := mgr.ApplyOperation(context.Background(), "sound-volume", map[string]any{"volume": 155}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "volume applied" {
		t.Fatalf("expected platform summary, got %q", summary)
	}
	if snapshot := mgr.Snapshot(); snapshot.SoundVolume != 100 {
		t.Fatalf("expected clamped volume to be 100, got %d", snapshot.SoundVolume)
	}
}

func TestSoundPlaybackReceivesExistingVolumeState(t *testing.T) {
	mgr := NewManager(ManagerOptions{})
	mgr.mu.Lock()
	mgr.state.SoundVolume = 25
	mgr.mu.Unlock()

	fake := &fakePlatformService{}
	fake.fn = func(ctx context.Context, operation string, metadata map[string]any, state State) (string, error) {
		if operation != "sound-playback" {
			return "", nil
		}
		if enabled, ok := metadata["enabled"].(bool); !ok || !enabled {
			t.Fatalf("expected enabled metadata to be true, got %v", metadata)
		}
		if state.SoundVolume != 25 {
			t.Fatalf("expected state volume to be 25, got %d", state.SoundVolume)
		}
		return "playback restored", nil
	}
	mgr.SetPlatformService(fake)

	summary, err := mgr.ApplyOperation(context.Background(), "sound-playback", map[string]any{"enabled": true}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "playback restored" {
		t.Fatalf("expected platform summary, got %q", summary)
	}
	if snapshot := mgr.Snapshot(); !snapshot.SoundPlayback {
		t.Fatalf("expected sound playback to be enabled")
	}
}
