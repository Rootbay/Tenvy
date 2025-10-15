package agent

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

func makeCommandResult(id string) protocol.CommandResult {
	return protocol.CommandResult{CommandID: id}
}

func TestEnqueueResultTrimsToMax(t *testing.T) {
	var a Agent
	for i := 0; i < maxBufferedResults; i++ {
		a.pendingResults = append(a.pendingResults, makeCommandResult(fmt.Sprintf("cmd-%d", i)))
	}

	extra := makeCommandResult("cmd-extra")
	a.enqueueResult(extra)

	if len(a.pendingResults) != maxBufferedResults {
		t.Fatalf("unexpected pending results length: got %d want %d", len(a.pendingResults), maxBufferedResults)
	}

	first := a.pendingResults[0].CommandID
	if first != "cmd-1" {
		t.Fatalf("unexpected first command id after trim: got %q want %q", first, "cmd-1")
	}

	last := a.pendingResults[len(a.pendingResults)-1].CommandID
	if last != extra.CommandID {
		t.Fatalf("expected last command to be new result: got %q want %q", last, extra.CommandID)
	}
}

func TestEnqueueResultsBatched(t *testing.T) {
	var a Agent
	initial := makeCommandResult("cmd-0")
	a.pendingResults = append(a.pendingResults, initial)

	batch := []protocol.CommandResult{
		makeCommandResult("cmd-1"),
		makeCommandResult("cmd-2"),
	}
	a.enqueueResults(batch)

	if len(a.pendingResults) != 3 {
		t.Fatalf("unexpected pending results length: got %d want %d", len(a.pendingResults), 3)
	}

	for idx, want := range []string{"cmd-0", "cmd-1", "cmd-2"} {
		if got := a.pendingResults[idx].CommandID; got != want {
			t.Fatalf("unexpected command id at index %d: got %q want %q", idx, got, want)
		}
	}
}

func TestEnqueueResultsLargeBatch(t *testing.T) {
	var a Agent
	batch := make([]protocol.CommandResult, maxBufferedResults+10)
	for i := range batch {
		batch[i] = makeCommandResult(fmt.Sprintf("cmd-%d", i))
	}

	a.enqueueResults(batch)

	if len(a.pendingResults) != maxBufferedResults {
		t.Fatalf("unexpected pending results length: got %d want %d", len(a.pendingResults), maxBufferedResults)
	}

	expectedFirst := fmt.Sprintf("cmd-%d", len(batch)-maxBufferedResults)
	if got := a.pendingResults[0].CommandID; got != expectedFirst {
		t.Fatalf("unexpected first command id after trimming batch: got %q want %q", got, expectedFirst)
	}

	expectedLast := fmt.Sprintf("cmd-%d", len(batch)-1)
	if got := a.pendingResults[len(a.pendingResults)-1].CommandID; got != expectedLast {
		t.Fatalf("unexpected last command id after trimming batch: got %q want %q", got, expectedLast)
	}
}

func TestShouldReRegister(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "unauthorized sentinel", err: protocol.ErrUnauthorized, want: true},
		{name: "wrapped unauthorized", err: fmt.Errorf("wrap: %w", protocol.ErrUnauthorized), want: true},
		{name: "http 404", err: &syncHTTPError{status: http.StatusNotFound, message: "status 404"}, want: true},
		{name: "http 410", err: &syncHTTPError{status: http.StatusGone, message: "status 410"}, want: true},
		{name: "http 500", err: &syncHTTPError{status: http.StatusInternalServerError, message: "status 500"}, want: false},
		{name: "generic", err: fmt.Errorf("boom"), want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldReRegister(tc.err); got != tc.want {
				t.Fatalf("unexpected result for %s: got %t want %t", tc.name, got, tc.want)
			}
		})
	}
}
