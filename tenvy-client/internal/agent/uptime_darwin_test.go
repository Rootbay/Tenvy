//go:build darwin

package agent

import (
	"errors"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestSystemUptime(t *testing.T) {
	originalSysctl := sysctlTimeval
	originalNow := timeNow
	t.Cleanup(func() {
		sysctlTimeval = originalSysctl
		timeNow = originalNow
	})

	boot := unix.Timeval{Sec: 100, Usec: 250000}
	fakeNow := time.Unix(200, 0)

	sysctlTimeval = func(string) (unix.Timeval, error) {
		return boot, nil
	}
	timeNow = func() time.Time {
		return fakeNow
	}

	uptime, err := systemUptime()
	if err != nil {
		t.Fatalf("systemUptime returned error: %v", err)
	}

	expected := fakeNow.Sub(time.Unix(int64(boot.Sec), int64(boot.Usec)*1000))
	if uptime != expected {
		t.Fatalf("expected uptime %s, got %s", expected, uptime)
	}
}

func TestSystemUptimeSysctlError(t *testing.T) {
	originalSysctl := sysctlTimeval
	originalNow := timeNow
	t.Cleanup(func() {
		sysctlTimeval = originalSysctl
		timeNow = originalNow
	})

	wantErr := errors.New("sysctl failure")

	sysctlTimeval = func(string) (unix.Timeval, error) {
		return unix.Timeval{}, wantErr
	}

	if _, err := systemUptime(); err != wantErr {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
}

func TestSystemUptimeFutureBoot(t *testing.T) {
	originalSysctl := sysctlTimeval
	originalNow := timeNow
	t.Cleanup(func() {
		sysctlTimeval = originalSysctl
		timeNow = originalNow
	})

	boot := unix.Timeval{Sec: 300, Usec: 0}
	now := time.Unix(200, 0)

	sysctlTimeval = func(string) (unix.Timeval, error) {
		return boot, nil
	}
	timeNow = func() time.Time {
		return now
	}

	if _, err := systemUptime(); err == nil {
		t.Fatal("expected error for future boot time")
	}
}
