package agent

import (
	"context"
	"errors"
	"io"
	"log"
	"testing"
	"time"
)

func TestRunWithWatchdogRestartsOnError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := RuntimeOptions{
		Logger:   log.New(io.Discard, "", 0),
		Watchdog: WatchdogConfig{Enabled: true, Interval: 5 * time.Millisecond},
	}

	runCount := 0
	runner := func(ctx context.Context, _ RuntimeOptions) error {
		runCount++
		if runCount == 1 {
			return errors.New("boom")
		}
		return nil
	}

	if err := runWithWatchdog(ctx, opts, runner); err != nil {
		t.Fatalf("watchdog returned error: %v", err)
	}

	if runCount != 2 {
		t.Fatalf("expected runner to execute twice, got %d", runCount)
	}
}

func TestRunWithWatchdogRespectsContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	opts := RuntimeOptions{
		Logger:   log.New(io.Discard, "", 0),
		Watchdog: WatchdogConfig{Enabled: true, Interval: time.Millisecond},
	}

	runner := func(ctx context.Context, _ RuntimeOptions) error {
		return errors.New("failure")
	}

	if err := runWithWatchdog(ctx, opts, runner); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}
