package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

func makeCommandResult(id string) protocol.CommandResult {
	return protocol.CommandResult{CommandID: id}
}

func newTestAgentWithStore(t *testing.T, cacheSize, retention int) *Agent {
	t.Helper()
	store, err := newResultStore(resultStoreConfig{Path: t.TempDir(), Retention: retention})
	if err != nil {
		t.Fatalf("create result store: %v", err)
	}
	logger := log.New(io.Discard, "", 0)
	agent := &Agent{
		logger:          logger,
		resultStore:     store,
		resultCacheSize: cacheSize,
		pendingResults:  make([]protocol.CommandResult, 0, cacheSize),
	}
	agent.reloadResultCache()
	return agent
}

func TestEnqueueResultTrimsToMax(t *testing.T) {
	cacheSize := 5
	a := newTestAgentWithStore(t, cacheSize, 100)
	for i := 0; i < cacheSize; i++ {
		a.enqueueResult(makeCommandResult(fmt.Sprintf("cmd-%d", i)))
	}

	extra := makeCommandResult("cmd-extra")
	a.enqueueResult(extra)

	if len(a.pendingResults) != cacheSize {
		t.Fatalf("unexpected pending results length: got %d want %d", len(a.pendingResults), cacheSize)
	}

	first := a.pendingResults[0].CommandID
	if first != "cmd-1" {
		t.Fatalf("unexpected first command id after trim: got %q want %q", first, "cmd-1")
	}

	last := a.pendingResults[len(a.pendingResults)-1].CommandID
	if last != extra.CommandID {
		t.Fatalf("expected last command to be new result: got %q want %q", last, extra.CommandID)
	}

	all, err := a.resultStore.All()
	if err != nil {
		t.Fatalf("unexpected error reading store: %v", err)
	}
	if len(all) != cacheSize+1 {
		t.Fatalf("expected store to retain all results, got %d", len(all))
	}
}

func TestEnqueueResultsBatched(t *testing.T) {
	a := newTestAgentWithStore(t, 5, 100)
	initial := makeCommandResult("cmd-0")
	a.enqueueResult(initial)

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

	all, err := a.resultStore.All()
	if err != nil {
		t.Fatalf("unexpected error reading store: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected store to contain 3 results, got %d", len(all))
	}
}

func TestEnqueueResultsLargeBatch(t *testing.T) {
	cacheSize := 10
	a := newTestAgentWithStore(t, cacheSize, 200)
	batch := make([]protocol.CommandResult, cacheSize+10)
	for i := range batch {
		batch[i] = makeCommandResult(fmt.Sprintf("cmd-%d", i))
	}

	a.enqueueResults(batch)

	if len(a.pendingResults) != cacheSize {
		t.Fatalf("unexpected pending results length: got %d want %d", len(a.pendingResults), cacheSize)
	}

	expectedFirst := fmt.Sprintf("cmd-%d", len(batch)-cacheSize)
	if got := a.pendingResults[0].CommandID; got != expectedFirst {
		t.Fatalf("unexpected first command id after trimming batch: got %q want %q", got, expectedFirst)
	}

	expectedLast := fmt.Sprintf("cmd-%d", len(batch)-1)
	if got := a.pendingResults[len(a.pendingResults)-1].CommandID; got != expectedLast {
		t.Fatalf("unexpected last command id after trimming batch: got %q want %q", got, expectedLast)
	}

	results, err := a.resultStore.All()
	if err != nil {
		t.Fatalf("unexpected error reading store: %v", err)
	}
	if len(results) != len(batch) {
		t.Fatalf("expected store to keep full batch, got %d want %d", len(results), len(batch))
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

	store, err := newResultStore(resultStoreConfig{Path: t.TempDir(), Retention: 100})
	if err != nil {
		t.Fatalf("create result store: %v", err)
	}
	agent := &Agent{
		baseURL:         server.URL,
		client:          server.Client(),
		logger:          log.New(io.Discard, "", 0),
		pendingResults:  make([]protocol.CommandResult, 0, len(expectedIDs)),
		resultStore:     store,
		resultCacheSize: len(expectedIDs),
		buildVersion:    "test",
	}
	agent.reloadResultCache()

	for _, id := range expectedIDs {
		agent.enqueueResult(makeCommandResult(id))
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
	if agent.resultStore != nil {
		if count, err := agent.resultStore.Count(); err != nil {
			t.Fatalf("result store count error: %v", err)
		} else if count != 0 {
			t.Fatalf("expected result store to be empty after sync, got %d", count)
		}
	}
}

func TestPerformSyncIncludesCustomHeadersAndCookies(t *testing.T) {
	t.Parallel()

	received := make(chan http.Header, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case received <- r.Header.Clone():
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(protocol.AgentSyncResponse{Config: protocol.AgentConfig{}})
	}))
	defer server.Close()

	agent := &Agent{
		id:             "agent-1",
		key:            "token-1",
		baseURL:        server.URL,
		client:         server.Client(),
		logger:         log.New(io.Discard, "", 0),
		requestHeaders: []CustomHeader{{Key: "X-Test", Value: "value"}},
		requestCookies: []CustomCookie{{Name: "session", Value: "abc"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if _, err := agent.performSync(ctx, statusOnline, nil); err != nil {
		t.Fatalf("performSync failed: %v", err)
	}

	select {
	case headers := <-received:
		if got := headers.Get("X-Test"); got != "value" {
			t.Fatalf("expected custom header to be set, got %q", got)
		}
		cookieHeader := headers.Get("Cookie")
		if cookieHeader == "" || !strings.Contains(cookieHeader, "session=abc") {
			t.Fatalf("expected custom cookie to be present, got %q", cookieHeader)
		}
	case <-time.After(time.Second):
		t.Fatalf("no sync request captured")
	}
}

func TestUserAgentDefaultFingerprint(t *testing.T) {
	t.Setenv("LC_ALL", "de_DE.UTF-8")
	agent := &Agent{
		buildVersion:             "1.2.3",
		metadata:                 protocol.AgentMetadata{Version: "1.2.3"},
		userAgentAutogenDisabled: false,
	}
	got := agent.userAgent()
	if strings.HasPrefix(got, "tenvy-client/") {
		t.Fatalf("expected fingerprinted user agent, got %q", got)
	}
	meta := currentUserAgentMetadata()
	want := generateUserAgentFromFingerprint(defaultUserAgentFingerprint(), meta)
	if want == "" {
		t.Fatalf("default fingerprint returned empty user agent")
	}
	if got != want {
		t.Fatalf("unexpected default user agent: got %q want %q", got, want)
	}
}

func TestUserAgentOverrideTakesPrecedence(t *testing.T) {
	agent := &Agent{
		buildVersion:      "1.2.3",
		metadata:          protocol.AgentMetadata{},
		userAgentOverride: "Custom-UA",
	}
	if got := agent.userAgent(); got != "Custom-UA" {
		t.Fatalf("expected override to be returned, got %q", got)
	}
}

func TestUserAgentAutoDisabled(t *testing.T) {
	agent := &Agent{
		buildVersion:             "1.2.3",
		metadata:                 protocol.AgentMetadata{},
		userAgentAutogenDisabled: true,
	}
	if got := agent.userAgent(); got != "" {
		t.Fatalf("expected auto-generation to be disabled, got %q", got)
	}
}

func TestUserAgentCustomFingerprint(t *testing.T) {
	agent := &Agent{
		buildVersion:         "1.2.3",
		metadata:             protocol.AgentMetadata{},
		userAgentFingerprint: userAgentFingerprintFirefoxLinux,
	}
	meta := currentUserAgentMetadata()
	want := generateUserAgentFromFingerprint(userAgentFingerprintFirefoxLinux, meta)
	if got := agent.userAgent(); got != want {
		t.Fatalf("unexpected user agent for custom fingerprint: got %q want %q", got, want)
	}
}
