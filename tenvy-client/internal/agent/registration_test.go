package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

func TestIsTemporaryNetworkError(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("boom"),
			expected: false,
		},
		{
			name:     "net error timeout",
			err:      &net.DNSError{IsTemporary: true},
			expected: true,
		},
		{
			name:     "op error",
			err:      &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("refused")},
			expected: true,
		},
		{
			name:     "url timeout",
			err:      &url.Error{Err: timeoutNetworkError{}},
			expected: true,
		},
		{
			name:     "url nested op error",
			err:      &url.Error{Err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("refused")}},
			expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if actual := isTemporaryNetworkError(tc.err); actual != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, actual)
			}
		})
	}
}

type timeoutNetworkError struct{}

func (timeoutNetworkError) Error() string   { return "timeout" }
func (timeoutNetworkError) Timeout() bool   { return true }
func (timeoutNetworkError) Temporary() bool { return true }

func TestRetryAfterDuration(t *testing.T) {
	t.Parallel()

	resp := &http.Response{Header: make(http.Header)}
	if got := retryAfterDuration(resp); got != 0 {
		t.Fatalf("expected zero duration for missing header, got %s", got)
	}

	resp.Header.Set("Retry-After", "5")
	if got := retryAfterDuration(resp); got != 5*time.Second {
		t.Fatalf("expected 5s duration, got %s", got)
	}

	resp.Header.Set("Retry-After", "-3")
	if got := retryAfterDuration(resp); got != 0 {
		t.Fatalf("expected zero duration for negative retry-after, got %s", got)
	}

	when := time.Now().Add(2 * time.Second)
	resp.Header.Set("Retry-After", when.UTC().Format(http.TimeFormat))
	if got := retryAfterDuration(resp); got < time.Second || got > 3*time.Second {
		t.Fatalf("expected duration near 2s, got %s", got)
	}

	resp.Header.Set("Retry-After", "invalid")
	if got := retryAfterDuration(resp); got != 0 {
		t.Fatalf("expected zero duration for invalid value, got %s", got)
	}
}

func TestIsTemporaryStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		status   int
		expected bool
	}{
		{status: http.StatusOK, expected: false},
		{status: http.StatusBadRequest, expected: false},
		{status: http.StatusTooManyRequests, expected: true},
		{status: http.StatusRequestTimeout, expected: true},
		{status: http.StatusTooEarly, expected: true},
		{status: http.StatusInternalServerError, expected: true},
	}

	for _, tc := range cases {
		if actual := isTemporaryStatus(tc.status); actual != tc.expected {
			t.Fatalf("status %d: expected %v, got %v", tc.status, tc.expected, actual)
		}
	}
}

func TestRegisterAgentWithRetryHonoursRetryAfter(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	metadata := protocol.AgentMetadata{Version: "test"}

	wait := time.Second
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("please retry"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(protocol.AgentRegistrationResponse{
			AgentID:    "agent",
			AgentKey:   "key",
			Config:     protocol.AgentConfig{},
			Commands:   nil,
			ServerTime: time.Now().UTC().Format(time.RFC3339Nano),
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	_, err := registerAgentWithRetry(ctx, logger, server.Client(), server.URL, "", metadata, time.Second, nil, nil)
	if err != nil {
		t.Fatalf("registerAgentWithRetry returned error: %v", err)
	}

	elapsed := time.Since(start)
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	if elapsed < 900*time.Millisecond {
		t.Fatalf("expected wait near %s, got %s", wait, elapsed)
	}
	if elapsed > 1200*time.Millisecond {
		t.Fatalf("expected wait near %s, got %s", wait, elapsed)
	}
}

func TestRegisterAgentWithRetryClampsWaitToContextDeadline(t *testing.T) {
	metadata := protocol.AgentMetadata{Version: "test"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()

	var buf strings.Builder
	logger := log.New(&buf, "", 0)

	start := time.Now()
	_, err := registerAgentWithRetry(ctx, logger, server.Client(), server.URL, "", metadata, time.Second, nil, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}

	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Fatalf("registerAgentWithRetry waited too long: %s", elapsed)
	}

	logs := buf.String()
	if strings.Contains(logs, "retrying registration in 1s") {
		t.Fatalf("expected wait to be clamped to context deadline, logs: %s", logs)
	}
	if !strings.Contains(logs, "retrying registration in") {
		t.Fatalf("expected retry log entry, logs: %s", logs)
	}
}
