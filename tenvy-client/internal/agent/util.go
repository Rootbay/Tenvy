package agent

import (
	"context"
	"time"
)

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func timestampNow() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// sleepContext pauses for the provided duration or until the context is
// cancelled. It centralises timer management to avoid repeated allocations and
// ensures the caller always observes context cancellation in a consistent way.
func sleepContext(ctx context.Context, d time.Duration) error {
	if ctx == nil {
		// Mirror the behaviour of time.After by panicking rather than
		// silently succeeding with a nil context.
		panic("sleepContext called with nil context")
	}

	if d <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
