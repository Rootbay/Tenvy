package clientchat

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

func TestSupervisorRespawnsSessionOnCrash(t *testing.T) {
	supervisor := NewSupervisor(Config{})
	sessionID, created := supervisor.ensureSession("")
	if sessionID == "" || !created {
		t.Fatalf("expected supervisor to create session, got id=%q created=%v", sessionID, created)
	}

	supervisor.mu.Lock()
	original := supervisor.session
	supervisor.mu.Unlock()
	if original == nil {
		t.Fatal("expected session to be initialized")
	}

	original.terminate(reasonCrash)

	deadline := time.Now().Add(200 * time.Millisecond)
	for {
		supervisor.mu.Lock()
		replacement := supervisor.session
		supervisor.mu.Unlock()
		if replacement != nil && replacement != original {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("supervisor did not respawn session after crash")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSupervisorStopSessionPreventsRespawn(t *testing.T) {
	supervisor := NewSupervisor(Config{})
	sessionID, _ := supervisor.ensureSession("")
	supervisor.mu.Lock()
	current := supervisor.session
	supervisor.mu.Unlock()
	if current == nil {
		t.Fatal("expected session to be initialized")
	}

	if err := supervisor.stopSession(sessionID); err != nil {
		t.Fatalf("stopSession error: %v", err)
	}

	current.terminate(reasonCrash)
	time.Sleep(20 * time.Millisecond)
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	if supervisor.session != nil {
		t.Fatal("expected supervisor session to remain nil after stop")
	}
}

func TestSupervisorHandleCommandLifecycle(t *testing.T) {
	supervisor := NewSupervisor(Config{})
	payload, err := json.Marshal(protocol.ClientChatCommandPayload{Action: "start"})
	if err != nil {
		t.Fatalf("marshal start payload: %v", err)
	}
	startResult := supervisor.HandleCommand(context.Background(), protocol.Command{ID: "start", Payload: payload})
	if !startResult.Success {
		t.Fatalf("start command failed: %v", startResult.Error)
	}

	supervisor.mu.Lock()
	active := supervisor.session
	supervisor.mu.Unlock()
	if active == nil {
		t.Fatal("expected active session after start")
	}

	stopPayload, err := json.Marshal(protocol.ClientChatCommandPayload{Action: "stop", SessionID: active.id})
	if err != nil {
		t.Fatalf("marshal stop payload: %v", err)
	}

	stopResult := supervisor.HandleCommand(context.Background(), protocol.Command{ID: "stop", Payload: stopPayload})
	if !stopResult.Success {
		t.Fatalf("stop command failed: %v", stopResult.Error)
	}

	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	if supervisor.session != nil {
		t.Fatal("expected session to be cleared after stop command")
	}
}

func TestSupervisorIgnoresUnstoppableDisable(t *testing.T) {
	supervisor := NewSupervisor(Config{})
	if _, created := supervisor.ensureSession(""); !created {
		t.Fatal("expected session creation")
	}

	falseValue := false
	payload, err := json.Marshal(protocol.ClientChatCommandPayload{
		Action: "configure",
		Features: &protocol.ClientChatFeatureFlags{
			Unstoppable: &falseValue,
		},
	})
	if err != nil {
		t.Fatalf("marshal configure payload: %v", err)
	}

	result := supervisor.HandleCommand(context.Background(), protocol.Command{ID: "configure", Payload: payload})
	if !result.Success {
		t.Fatalf("configure command failed: %v", result.Error)
	}

	supervisor.mu.Lock()
	unstoppable := supervisor.unstoppable
	supervisor.mu.Unlock()
	if !unstoppable {
		t.Fatal("expected unstoppable flag to remain true")
	}
}

func TestSupervisorDeliversOperatorMessagesToHooks(t *testing.T) {
	supervisor := NewSupervisor(Config{})
	if _, created := supervisor.ensureSession(""); !created {
		t.Fatal("expected session creation")
	}

	deliveries := make(chan OperatorMessageDelivery, 1)
	cancel, err := supervisor.RegisterDeliveryConsumer("ui", OperatorMessageConsumerFunc(func(ctx context.Context, delivery OperatorMessageDelivery) {
		deliveries <- delivery
		delivery.Ack()
	}))
	if err != nil {
		t.Fatalf("register delivery consumer: %v", err)
	}
	defer cancel()

	payload, err := json.Marshal(protocol.ClientChatCommandPayload{
		Action: "send-message",
		Message: &protocol.ClientChatCommandMessage{
			Body: "Hello there",
		},
	})
	if err != nil {
		t.Fatalf("marshal send payload: %v", err)
	}

	result := supervisor.HandleCommand(context.Background(), protocol.Command{ID: "send", Payload: payload})
	if !result.Success {
		t.Fatalf("send command failed: %v", result.Error)
	}

	select {
	case delivery := <-deliveries:
		if delivery.Message.Body != "Hello there" {
			t.Fatalf("unexpected delivery body: %q", delivery.Message.Body)
		}
		if delivery.SessionID == "" {
			t.Fatal("expected session identifier on delivery")
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("did not receive operator message delivery")
	}

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if supervisor.ensureRouter().pendingLen() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pending := supervisor.ensureRouter().pendingLen(); pending != 0 {
		t.Fatalf("expected no pending messages, have %d", pending)
	}

	followup := make(chan OperatorMessageDelivery, 1)
	cancelFollowup, err := supervisor.RegisterDeliveryConsumer("audit", OperatorMessageConsumerFunc(func(ctx context.Context, delivery OperatorMessageDelivery) {
		followup <- delivery
	}))
	if err != nil {
		t.Fatalf("register followup consumer: %v", err)
	}
	defer cancelFollowup()

	select {
	case <-followup:
		t.Fatal("unexpected delivery for cleared queue")
	case <-time.After(150 * time.Millisecond):
	}
}

func TestSupervisorPendingMessagesPersistUntilAck(t *testing.T) {
	supervisor := NewSupervisor(Config{})
	if _, created := supervisor.ensureSession(""); !created {
		t.Fatal("expected session creation")
	}

	firstDelivery := make(chan OperatorMessageDelivery, 1)
	cancelFirst, err := supervisor.RegisterDeliveryConsumer("first", OperatorMessageConsumerFunc(func(ctx context.Context, delivery OperatorMessageDelivery) {
		firstDelivery <- delivery
	}))
	if err != nil {
		t.Fatalf("register first consumer: %v", err)
	}
	defer cancelFirst()

	payload, err := json.Marshal(protocol.ClientChatCommandPayload{
		Action:  "send-message",
		Message: &protocol.ClientChatCommandMessage{Body: "queued"},
	})
	if err != nil {
		t.Fatalf("marshal send payload: %v", err)
	}
	result := supervisor.HandleCommand(context.Background(), protocol.Command{ID: "send", Payload: payload})
	if !result.Success {
		t.Fatalf("send command failed: %v", result.Error)
	}

	select {
	case <-firstDelivery:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("first consumer did not receive message")
	}

	if pending := supervisor.ensureRouter().pendingLen(); pending == 0 {
		t.Fatal("expected message to remain pending until acked")
	}

	secondDelivery := make(chan OperatorMessageDelivery, 1)
	cancelSecond, err := supervisor.RegisterDeliveryConsumer("second", OperatorMessageConsumerFunc(func(ctx context.Context, delivery OperatorMessageDelivery) {
		secondDelivery <- delivery
		delivery.Ack()
	}))
	if err != nil {
		t.Fatalf("register second consumer: %v", err)
	}
	defer cancelSecond()

	select {
	case <-secondDelivery:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("second consumer did not receive pending message")
	}

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if supervisor.ensureRouter().pendingLen() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pending := supervisor.ensureRouter().pendingLen(); pending != 0 {
		t.Fatalf("expected pending messages to be cleared, have %d", pending)
	}
}

func TestSupervisorSubmitClientMessage(t *testing.T) {
	stub := &stubHTTPClient{}
	cfg := Config{
		AgentID: "agent-123",
		BaseURL: "https://controller.example",
		AuthKey: "secret",
		Client:  stub,
	}
	supervisor := NewSupervisor(cfg)
	if err := supervisor.SubmitClientMessage(context.Background(), "hello operator"); err != nil {
		t.Fatalf("submit client message: %v", err)
	}

	stub.mu.Lock()
	defer stub.mu.Unlock()
	if len(stub.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(stub.requests))
	}
	req := stub.requests[0]
	if req.Method != http.MethodPost {
		t.Fatalf("expected POST request, got %s", req.Method)
	}
	if !strings.Contains(req.URL.Path, "/api/agents/agent-123/chat/messages") {
		t.Fatalf("unexpected request path: %s", req.URL.Path)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("expected request body")
	}
}

type stubHTTPClient struct {
	mu       sync.Mutex
	requests []*http.Request
}

func (s *stubHTTPClient) Do(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	_ = req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(body))

	s.mu.Lock()
	s.requests = append(s.requests, req)
	s.mu.Unlock()

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("{}")),
	}, nil
}
