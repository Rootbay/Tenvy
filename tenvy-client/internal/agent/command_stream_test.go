package agent

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
	"nhooyr.io/websocket"
)

func makeTestAgent(baseURL string, client *http.Client, router *commandRouter) *Agent {
	return &Agent{
		id:             "agent-1",
		key:            "key-1",
		baseURL:        baseURL,
		client:         client,
		config:         protocol.AgentConfig{PollIntervalMs: 50, MaxBackoffMs: 200, JitterRatio: 0},
		logger:         log.New(io.Discard, "", 0),
		pendingResults: make([]protocol.CommandResult, 0, 4),
		startTime:      time.Now(),
		buildVersion:   "test",
		commands:       router,
	}
}

func TestCommandStreamDeliversImmediately(t *testing.T) {
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

	var connMu sync.Mutex
	var sessionConn *websocket.Conn
	sessionReady := make(chan struct{})

	mux := http.NewServeMux()
	mux.HandleFunc("/api/agents/agent-1/session", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer key-1" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
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
		close(sessionReady)
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

	srv := httptest.NewServer(mux)
	defer srv.Close()

	agent := makeTestAgent(srv.URL, srv.Client(), router)

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

	srv := httptest.NewServer(mux)
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

	srv := httptest.NewServer(mux)
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

func TestStopRemoteDesktopInputWorkerSignalsShutdown(t *testing.T) {
	agent := &Agent{
		logger:  log.New(io.Discard, "", 0),
		modules: &moduleRegistry{remote: &remoteDesktopModule{}},
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
		modules: &moduleRegistry{remote: &remoteDesktopModule{}},
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
		modules: &moduleRegistry{remote: &remoteDesktopModule{}},
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
