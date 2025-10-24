package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	remotedesktop "github.com/rootbay/tenvy-client/internal/modules/control/remotedesktop"
	"github.com/rootbay/tenvy-client/internal/protocol"
	"nhooyr.io/websocket"
)

type stubRemoteDesktopEngine struct{}

func (stubRemoteDesktopEngine) Configure(remotedesktop.Config) error { return nil }
func (stubRemoteDesktopEngine) StartSession(context.Context, remotedesktop.RemoteDesktopCommandPayload) error {
	return nil
}
func (stubRemoteDesktopEngine) StopSession(string) error { return nil }
func (stubRemoteDesktopEngine) UpdateSession(remotedesktop.RemoteDesktopCommandPayload) error {
	return nil
}
func (stubRemoteDesktopEngine) HandleInput(context.Context, remotedesktop.RemoteDesktopCommandPayload) error {
	return nil
}
func (stubRemoteDesktopEngine) DeliverFrame(context.Context, remotedesktop.RemoteDesktopFramePacket) error {
	return nil
}
func (stubRemoteDesktopEngine) Shutdown() {}

type recordingAppVncHandler struct {
	mu     sync.Mutex
	bursts []protocol.AppVncInputBurst
}

func (r *recordingAppVncHandler) HandleInputBurst(_ context.Context, burst protocol.AppVncInputBurst) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copyBurst := protocol.AppVncInputBurst{
		SessionID: burst.SessionID,
		Sequence:  burst.Sequence,
		Events:    append([]protocol.AppVncInputEvent(nil), burst.Events...),
	}
	r.bursts = append(r.bursts, copyBurst)
	return nil
}

func (r *recordingAppVncHandler) snapshot() []protocol.AppVncInputBurst {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]protocol.AppVncInputBurst, len(r.bursts))
	copy(out, r.bursts)
	for i, burst := range out {
		if len(burst.Events) > 0 {
			events := append([]protocol.AppVncInputEvent(nil), burst.Events...)
			out[i].Events = events
		}
	}
	return out
}

func makeTestAgent(baseURL string, client *http.Client, router *commandRouter) *Agent {
	return &Agent{
		id:                   "agent-1",
		key:                  "key-1",
		baseURL:              baseURL,
		client:               client,
		config:               protocol.AgentConfig{PollIntervalMs: 50, MaxBackoffMs: 200, JitterRatio: 0},
		logger:               log.New(io.Discard, "", 0),
		pendingResults:       make([]protocol.CommandResult, 0, 4),
		startTime:            time.Now(),
		metadata:             protocol.AgentMetadata{Version: "test"},
		buildVersion:         "test",
		userAgentFingerprint: defaultUserAgentFingerprint(),
		commands:             router,
	}
}

func TestCommandStreamDeliversImmediately(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	t.Setenv("LC_ALL", "en_US.UTF-8")

	executed := make(chan protocol.Command, 1)
	router := newCommandRouter()
	if err := router.register("ping", func(_ context.Context, _ *Agent, cmd protocol.Command) protocol.CommandResult {
		executed <- cmd
		return newSuccessResult(cmd.ID, "ok")
	}); err != nil {
		t.Fatalf("failed to register command handler: %v", err)
	}

	var connMu sync.Mutex
	var sessionConn *websocket.Conn
	sessionReady := make(chan struct{})
	var sessionOnce sync.Once

	var tokenMu sync.Mutex
	var issuedToken string

	var expectedUserAgent string

	mux := http.NewServeMux()
	mux.HandleFunc("/api/agents/agent-1/session-token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer key-1" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if got := r.Header.Get("User-Agent"); got != expectedUserAgent {
			http.Error(w, "invalid user agent", http.StatusBadRequest)
			return
		}
		token := fmt.Sprintf("session-%d", time.Now().UnixNano())
		tokenMu.Lock()
		issuedToken = token
		tokenMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessionTokenResponse{
			Token:     token,
			ExpiresAt: time.Now().Add(30 * time.Second).UTC().Format(time.RFC3339Nano),
		})
	})
	mux.HandleFunc("/api/agents/agent-1/session", func(w http.ResponseWriter, r *http.Request) {
		tokenMu.Lock()
		token := issuedToken
		tokenMu.Unlock()
		if got := r.Header.Get(sessionTokenHeader); got != token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Authorization") != "" {
			http.Error(w, "authorization header unexpected", http.StatusUnauthorized)
			return
		}
		if got := r.Header.Get("User-Agent"); got != expectedUserAgent {
			http.Error(w, "invalid user agent", http.StatusBadRequest)
			return
		}
		if r.Header.Get("Sec-WebSocket-Protocol") != protocol.CommandStreamSubprotocol {
			http.Error(w, "protocol", http.StatusBadRequest)
			return
		}
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{protocol.CommandStreamSubprotocol}})
		if err != nil {
			t.Errorf("accept websocket: %v", err)
			return
		}
		connMu.Lock()
		sessionConn = c
		connMu.Unlock()
		sessionOnce.Do(func() {
			close(sessionReady)
		})
	})
	mux.HandleFunc("/api/agents/agent-1/sync", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != expectedUserAgent {
			http.Error(w, "invalid user agent", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(protocol.AgentSyncResponse{
			AgentID:    "agent-1",
			Commands:   nil,
			Config:     protocol.AgentConfig{},
			ServerTime: time.Now().UTC().Format(time.RFC3339Nano),
		})
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	agent := makeTestAgent(srv.URL, srv.Client(), router)
	agent.userAgentFingerprint = userAgentFingerprintEdgeWindows
	expectedUserAgent = agent.userAgent()

	go agent.runCommandStream(ctx)

	select {
	case <-sessionReady:
	case <-ctx.Done():
		t.Fatalf("command stream did not connect: %v", ctx.Err())
	}

	connMu.Lock()
	c := sessionConn
	connMu.Unlock()
	if c == nil {
		t.Fatalf("session connection not captured")
	}
	defer c.Close(websocket.StatusNormalClosure, "test complete")

	payload := protocol.CommandEnvelope{
		Type: "command",
		Command: &protocol.Command{
			ID:        "cmd-1",
			Name:      "ping",
			Payload:   json.RawMessage(`{"message":"hi"}`),
			CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal command: %v", err)
	}

	if err := c.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write command: %v", err)
	}

	select {
	case cmd := <-executed:
		if cmd.Name != "ping" {
			t.Fatalf("unexpected command name: %s", cmd.Name)
		}
	case <-ctx.Done():
		t.Fatalf("command was not executed: %v", ctx.Err())
	}
}

func TestCommandStreamDispatchesAppVncInput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	router := newCommandRouter()
	handler := &recordingAppVncHandler{}

	var connMu sync.Mutex
	var sessionConn *websocket.Conn
	sessionReady := make(chan struct{})
	var sessionOnce sync.Once

	var tokenMu sync.Mutex
	var issuedToken string

	mux := http.NewServeMux()
	mux.HandleFunc("/api/agents/agent-1/session-token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		token := fmt.Sprintf("session-%d", time.Now().UnixNano())
		tokenMu.Lock()
		issuedToken = token
		tokenMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessionTokenResponse{
			Token:     token,
			ExpiresAt: time.Now().Add(30 * time.Second).UTC().Format(time.RFC3339Nano),
		})
	})
	mux.HandleFunc("/api/agents/agent-1/session", func(w http.ResponseWriter, r *http.Request) {
		tokenMu.Lock()
		token := issuedToken
		tokenMu.Unlock()
		if got := r.Header.Get(sessionTokenHeader); got != token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{protocol.CommandStreamSubprotocol}})
		if err != nil {
			t.Errorf("accept websocket: %v", err)
			return
		}
		connMu.Lock()
		sessionConn = c
		connMu.Unlock()
		sessionOnce.Do(func() {
			close(sessionReady)
		})
	})
	mux.HandleFunc("/api/agents/agent-1/sync", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(protocol.AgentSyncResponse{AgentID: "agent-1", ServerTime: time.Now().UTC().Format(time.RFC3339Nano)})
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	agent := makeTestAgent(srv.URL, srv.Client(), router)
	agent.modules = &moduleManager{appVnc: handler}

	go agent.runCommandStream(ctx)

	select {
	case <-sessionReady:
	case <-ctx.Done():
		t.Fatalf("command stream did not connect: %v", ctx.Err())
	}

	connMu.Lock()
	c := sessionConn
	connMu.Unlock()
	if c == nil {
		t.Fatalf("session connection not captured")
	}
	defer c.Close(websocket.StatusNormalClosure, "test complete")

	burst := protocol.AppVncInputBurst{
		SessionID: "session-1",
		Sequence:  42,
		Events: []protocol.AppVncInputEvent{{
			Type:       protocol.AppVncInputPointerMove,
			CapturedAt: time.Now().UnixMilli(),
			X:          0.5,
			Y:          0.25,
			Normalized: true,
		}},
	}
	payload := protocol.CommandEnvelope{
		Type:        "app-vnc-input",
		AppVncInput: &burst,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal app-vnc input: %v", err)
	}

	if err := c.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write app-vnc input: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("app-vnc burst not delivered")
		default:
			bursts := handler.snapshot()
			if len(bursts) > 0 {
				received := bursts[0]
				if received.SessionID != burst.SessionID {
					t.Fatalf("unexpected session id: %s", received.SessionID)
				}
				if received.Sequence != burst.Sequence {
					t.Fatalf("unexpected sequence: %d", received.Sequence)
				}
				if len(received.Events) != len(burst.Events) {
					t.Fatalf("unexpected event count: %d", len(received.Events))
				}
				if received.Events[0].Type != burst.Events[0].Type {
					t.Fatalf("unexpected event type: %s", received.Events[0].Type)
				}
				if received.Events[0].X != burst.Events[0].X || received.Events[0].Y != burst.Events[0].Y {
					t.Fatalf("unexpected pointer coordinates: %v,%v", received.Events[0].X, received.Events[0].Y)
				}
				if !received.Events[0].Normalized {
					t.Fatalf("expected normalized flag")
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestCommandStreamFallsBackToSync(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	executed := make(chan protocol.Command, 1)
	router := newCommandRouter()
	if err := router.register("ping", func(_ context.Context, _ *Agent, cmd protocol.Command) protocol.CommandResult {
		executed <- cmd
		return newSuccessResult(cmd.ID, "ok")
	}); err != nil {
		t.Fatalf("failed to register command handler: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/agents/agent-1/session-token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessionTokenResponse{
			Token:     "fallback-token",
			ExpiresAt: time.Now().Add(30 * time.Second).UTC().Format(time.RFC3339Nano),
		})
	})
	mux.HandleFunc("/api/agents/agent-1/session", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	syncCount := 0
	mux.HandleFunc("/api/agents/agent-1/sync", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		syncCount++
		resp := protocol.AgentSyncResponse{
			AgentID:    "agent-1",
			Config:     protocol.AgentConfig{PollIntervalMs: 50, MaxBackoffMs: 200},
			ServerTime: time.Now().UTC().Format(time.RFC3339Nano),
		}
		if syncCount == 1 {
			resp.Commands = []protocol.Command{{
				ID:        "cmd-sync",
				Name:      "ping",
				Payload:   json.RawMessage(`{"message":"sync"}`),
				CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}}
		}
		json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	agent := makeTestAgent(srv.URL, srv.Client(), router)
	agent.timing = TimingOverride{PollInterval: 50 * time.Millisecond, MaxBackoff: 100 * time.Millisecond}

	go agent.runCommandStream(ctx)

	if err := agent.sync(ctx, statusOnline); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	select {
	case cmd := <-executed:
		if cmd.ID != "cmd-sync" {
			t.Fatalf("unexpected command id: %s", cmd.ID)
		}
	case <-ctx.Done():
		t.Fatalf("command from sync not executed: %v", ctx.Err())
	}
}

func TestCommandStreamRequestsReconnectOnUnauthorized(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	router := newCommandRouter()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/agents/agent-1/session-token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessionTokenResponse{
			Token:     "unauthorized-token",
			ExpiresAt: time.Now().Add(30 * time.Second).UTC().Format(time.RFC3339Nano),
		})
	})
	mux.HandleFunc("/api/agents/agent-1/session", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	mux.HandleFunc("/api/agents/agent-1/sync", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(protocol.AgentSyncResponse{
			AgentID:    "agent-1",
			Config:     protocol.AgentConfig{},
			ServerTime: time.Now().UTC().Format(time.RFC3339Nano),
		})
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	agent := makeTestAgent(srv.URL, srv.Client(), router)

	go agent.runCommandStream(ctx)

	deadline := time.After(500 * time.Millisecond)
	for {
		if agent.connectionFlag.Load() == connectionDirectiveReconnect {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("context ended before reconnect flag set: %v", ctx.Err())
		case <-deadline:
			t.Fatalf("reconnect flag not set in time")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestCommandStreamUsesCustomUserAgent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	router := newCommandRouter()

	const customUserAgent = "Custom-Agent/2.0"

	var tokenUserAgent string
	var sessionUserAgent string

	var tokenMu sync.Mutex
	var issuedToken string

	mux := http.NewServeMux()
	mux.HandleFunc("/api/agents/agent-1/session-token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		tokenMu.Lock()
		issuedToken = fmt.Sprintf("session-%d", time.Now().UnixNano())
		tokenMu.Unlock()
		tokenUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessionTokenResponse{
			Token:     issuedToken,
			ExpiresAt: time.Now().Add(30 * time.Second).UTC().Format(time.RFC3339Nano),
		})
	})
	mux.HandleFunc("/api/agents/agent-1/session", func(w http.ResponseWriter, r *http.Request) {
		tokenMu.Lock()
		token := issuedToken
		tokenMu.Unlock()
		if r.Header.Get(sessionTokenHeader) != token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		sessionUserAgent = r.Header.Get("User-Agent")
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{protocol.CommandStreamSubprotocol}})
		if err != nil {
			t.Fatalf("accept websocket: %v", err)
		}
		go func() {
			defer c.Close(websocket.StatusNormalClosure, "done")
			<-ctx.Done()
		}()
	})
	mux.HandleFunc("/api/agents/agent-1/sync", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(protocol.AgentSyncResponse{
			AgentID:    "agent-1",
			Commands:   nil,
			Config:     protocol.AgentConfig{},
			ServerTime: time.Now().UTC().Format(time.RFC3339Nano),
		})
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	agent := makeTestAgent(srv.URL, srv.Client(), router)
	agent.userAgentOverride = customUserAgent

	go agent.runCommandStream(ctx)

	select {
	case <-time.After(500 * time.Millisecond):
	case <-ctx.Done():
	}

	if tokenUserAgent != customUserAgent {
		t.Fatalf("expected session token request user agent %q, got %q", customUserAgent, tokenUserAgent)
	}
	if sessionUserAgent != customUserAgent {
		t.Fatalf("expected session request user agent %q, got %q", customUserAgent, sessionUserAgent)
	}

}

func TestStopRemoteDesktopInputWorkerSignalsShutdown(t *testing.T) {
	agent := &Agent{
		logger:  log.New(io.Discard, "", 0),
		modules: &moduleManager{remote: newRemoteDesktopModule(stubRemoteDesktopEngine{})},
	}

	queue := agent.ensureRemoteDesktopInputWorker()
	if queue == nil {
		t.Fatal("expected remote desktop input queue")
	}

	agent.stopRemoteDesktopInputWorker()
	agent.stopRemoteDesktopInputWorker()

	if !agent.remoteDesktopInputStopped.Load() {
		t.Fatal("expected remote desktop input worker to be marked stopped")
	}

	select {
	case <-agent.remoteDesktopInputStopCh:
	case <-time.After(time.Second):
		t.Fatal("remote desktop input stop signal not closed")
	}
}

func TestHandleRemoteDesktopInputAfterStopReturnsImmediately(t *testing.T) {
	agent := &Agent{
		logger:  log.New(io.Discard, "", 0),
		modules: &moduleManager{remote: newRemoteDesktopModule(stubRemoteDesktopEngine{})},
	}

	if agent.ensureRemoteDesktopInputWorker() == nil {
		t.Fatal("expected remote desktop input queue")
	}

	agent.stopRemoteDesktopInputWorker()

	done := make(chan struct{})
	go func() {
		defer close(done)
		agent.handleRemoteDesktopInput(context.Background(), protocol.RemoteDesktopInputBurst{
			Events: []protocol.RemoteDesktopInputEvent{{Type: "mouse"}},
		})
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handleRemoteDesktopInput did not return after stop")
	}
}

func TestStopRemoteDesktopInputWorkerBeforeStart(t *testing.T) {
	agent := &Agent{
		logger:  log.New(io.Discard, "", 0),
		modules: &moduleManager{remote: newRemoteDesktopModule(stubRemoteDesktopEngine{})},
	}

	agent.stopRemoteDesktopInputWorker()

	if !agent.remoteDesktopInputStopped.Load() {
		t.Fatal("expected remote desktop input worker to be marked stopped")
	}

	if queue := agent.ensureRemoteDesktopInputWorker(); queue != nil {
		t.Fatal("expected ensureRemoteDesktopInputWorker to return nil after stop")
	}

	if agent.remoteDesktopInputStopCh == nil {
		t.Fatal("expected stop signal to be initialized")
	}
}
