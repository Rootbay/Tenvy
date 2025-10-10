package agent

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSleepContextHonoursDuration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const delay = 15 * time.Millisecond
	start := time.Now()
	if err := sleepContext(ctx, delay); err != nil {
		t.Fatalf("sleepContext returned error: %v", err)
	}

	elapsed := time.Since(start)
	if elapsed < delay {
		t.Fatalf("sleepContext returned too early: %s < %s", elapsed, delay)
	}
}

func TestSleepContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := sleepContext(ctx, time.Hour)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation error, got %v", err)
	}
}

func TestSleepContextZeroDuration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sleepContext(ctx, 0); err != nil {
		t.Fatalf("expected nil error for zero duration, got %v", err)
	}
}
