package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestReRegisterPreservesPendingResults(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	expectedIDs := []string{"result-1", "result-2"}

	resultsCh := make(chan []protocol.CommandResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/agents/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		response := protocol.AgentRegistrationResponse{
			AgentID:  "agent-new",
			AgentKey: "key-new",
			Config:   protocol.AgentConfig{},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/api/agents/agent-new/sync", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer key-new" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var payload protocol.AgentSyncRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		select {
		case resultsCh <- payload.Results:
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(protocol.AgentSyncResponse{Config: protocol.AgentConfig{}}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	originalResolver := publicIPCache
	publicIPCache = &publicIPResolver{
		value:     "cached",
		expiresAt: time.Now().Add(time.Hour),
	}
	defer func() { publicIPCache = originalResolver }()

	agent := &Agent{
		baseURL:        server.URL,
		client:         server.Client(),
		logger:         log.New(io.Discard, "", 0),
		pendingResults: make([]protocol.CommandResult, 0, len(expectedIDs)),
		buildVersion:   "test",
	}

	for _, id := range expectedIDs {
		agent.pendingResults = append(agent.pendingResults, makeCommandResult(id))
	}

	if err := agent.reRegister(ctx); err != nil {
		t.Fatalf("reRegister returned error: %v", err)
	}

	if agent.id != "agent-new" {
		t.Fatalf("expected agent id to be refreshed: got %q want %q", agent.id, "agent-new")
	}
	if agent.key != "key-new" {
		t.Fatalf("expected agent key to be refreshed: got %q want %q", agent.key, "key-new")
	}

	if err := agent.sync(ctx, statusOnline); err != nil {
		t.Fatalf("sync returned error: %v", err)
	}

	select {
	case results := <-resultsCh:
		if len(results) != len(expectedIDs) {
			t.Fatalf("unexpected number of results submitted: got %d want %d", len(results), len(expectedIDs))
		}
		for idx, id := range expectedIDs {
			if results[idx].CommandID != id {
				t.Fatalf("unexpected result at index %d: got %q want %q", idx, results[idx].CommandID, id)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sync results")
	}

	if len(agent.pendingResults) != 0 {
		t.Fatalf("expected pending results to be consumed after sync, got %d", len(agent.pendingResults))
	}
}
