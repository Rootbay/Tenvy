package webcam

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

type recordedRequest struct {
	Method string
	URL    string
	Body   []byte
	Header http.Header
}

type fakeHTTPClient struct {
	mu       sync.Mutex
	requests []recordedRequest
	handler  func(req *http.Request, body []byte) (*http.Response, error)
}

func (c *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	req.Body.Close()

	clone := recordedRequest{
		Method: req.Method,
		URL:    req.URL.String(),
		Body:   append([]byte(nil), body...),
		Header: req.Header.Clone(),
	}

	c.mu.Lock()
	c.requests = append(c.requests, clone)
	handler := c.handler
	c.mu.Unlock()

	if handler != nil {
		return handler(req, body)
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}, nil
}

func (c *fakeHTTPClient) Requests() []recordedRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]recordedRequest, len(c.requests))
	copy(out, c.requests)
	return out
}

func waitForCondition(tb testing.TB, fn func() bool, timeout time.Duration) {
	tb.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !fn() {
		tb.Fatalf("condition not satisfied within %s", timeout)
	}
}

type testFrameSource struct {
	frames    chan framePacket
	startErr  error
	updateErr error
	mu        sync.Mutex
	updates   []*protocol.WebcamStreamSettings
	closed    bool
}

func (t *testFrameSource) Start(ctx context.Context) (<-chan framePacket, error) {
	if t.startErr != nil {
		return nil, t.startErr
	}
	out := make(chan framePacket)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case packet, ok := <-t.frames:
				if !ok {
					return
				}
				select {
				case out <- packet:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, nil
}

func (t *testFrameSource) ApplySettings(settings *protocol.WebcamStreamSettings) error {
	if t.updateErr != nil {
		return t.updateErr
	}
	t.mu.Lock()
	t.updates = append(t.updates, cloneStreamSettings(settings))
	t.mu.Unlock()
	return nil
}

func (t *testFrameSource) Close() error {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()
	return nil
}

func (t *testFrameSource) WasClosed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closed
}

func TestHandleCommandEnumeratePublishesInventory(t *testing.T) {
	original := captureWebcamInventory
	captureWebcamInventory = func() ([]protocol.WebcamDevice, string, error) {
		return []protocol.WebcamDevice{{ID: "/dev/video0", Label: "Integrated Camera"}}, "", nil
	}
	defer func() { captureWebcamInventory = original }()

	client := &fakeHTTPClient{}
	manager := NewManager(Config{AgentID: "agent-1", BaseURL: "https://controller.example", Client: client})
	fixed := time.Date(2024, 6, 18, 12, 30, 0, 0, time.UTC)
	manager.setNowFunc(func() time.Time { return fixed })

	payload := protocol.WebcamCommandPayload{Action: "enumerate", RequestID: "req-1"}
	data, _ := json.Marshal(payload)

	result := manager.HandleCommand(context.Background(), Command{ID: "cmd-1", Payload: data})
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	waitForCondition(t, func() bool { return len(client.Requests()) == 1 }, time.Second)
	requests := client.Requests()
	if requests[0].Method != http.MethodPost {
		t.Fatalf("expected POST, got %s", requests[0].Method)
	}
	if requests[0].URL != "https://controller.example/api/agents/agent-1/webcam/devices" {
		t.Fatalf("unexpected URL: %s", requests[0].URL)
	}

	var inventory protocol.WebcamDeviceInventory
	if err := json.Unmarshal(requests[0].Body, &inventory); err != nil {
		t.Fatalf("failed to decode inventory: %v", err)
	}
	if inventory.RequestID != "req-1" {
		t.Fatalf("expected request id req-1, got %s", inventory.RequestID)
	}
	if len(inventory.Devices) != 1 || inventory.Devices[0].ID != "/dev/video0" {
		t.Fatalf("unexpected devices: %+v", inventory.Devices)
	}
}

func TestWebcamStreamStartStopLifecycle(t *testing.T) {
	client := &fakeHTTPClient{}
	manager := NewManager(Config{AgentID: "agent-9", BaseURL: "https://controller.example", Client: client})
	manager.setNowFunc(func() time.Time { return time.Date(2024, 7, 2, 9, 0, 0, 0, time.UTC) })

	frames := make(chan framePacket, 1)
	source := &testFrameSource{frames: frames}
	manager.setFrameSourceFactory(func(deviceID string, settings *protocol.WebcamStreamSettings) (frameSource, error) {
		if deviceID != "camera-1" {
			t.Fatalf("unexpected device id: %s", deviceID)
		}
		return source, nil
	})

	startPayload := protocol.WebcamCommandPayload{Action: "start", SessionID: "session-1", DeviceID: "camera-1"}
	startData, _ := json.Marshal(startPayload)
	result := manager.HandleCommand(context.Background(), Command{ID: "cmd-start", Payload: startData})
	if !result.Success {
		t.Fatalf("start failed: %s", result.Error)
	}

	waitForCondition(t, func() bool {
		reqs := client.Requests()
		return len(reqs) >= 1 && reqs[0].Method == http.MethodPatch
	}, 2*time.Second)

	frames <- framePacket{Data: []byte{0x01, 0x02, 0x03}, MimeType: "image/jpeg", CapturedAt: time.Date(2024, 7, 2, 9, 0, 10, 0, time.UTC)}

	waitForCondition(t, func() bool { return len(client.Requests()) >= 2 }, 2*time.Second)
	requests := client.Requests()
	frameReq := requests[1]
	if frameReq.Method != http.MethodPost {
		t.Fatalf("expected POST for frame, got %s", frameReq.Method)
	}
	if frameReq.URL != "https://controller.example/api/agents/agent-9/webcam/sessions/session-1/frames" {
		t.Fatalf("unexpected frame URL: %s", frameReq.URL)
	}

	stopPayload := protocol.WebcamCommandPayload{Action: "stop", SessionID: "session-1"}
	stopData, _ := json.Marshal(stopPayload)
	stopResult := manager.HandleCommand(context.Background(), Command{ID: "cmd-stop", Payload: stopData})
	if !stopResult.Success {
		t.Fatalf("stop failed: %s", stopResult.Error)
	}

	waitForCondition(t, func() bool { return len(client.Requests()) >= 3 }, 2*time.Second)
	waitForCondition(t, func() bool { manager.mu.Lock(); defer manager.mu.Unlock(); return len(manager.sessions) == 0 }, 2*time.Second)

	if !source.WasClosed() {
		t.Fatalf("expected frame source to be closed")
	}
}

func TestWebcamStreamUpdateNegotiation(t *testing.T) {
	client := &fakeHTTPClient{}
	manager := NewManager(Config{AgentID: "agent-5", BaseURL: "https://controller.example", Client: client})

	frames := make(chan framePacket)
	source := &testFrameSource{frames: frames}
	manager.setFrameSourceFactory(func(deviceID string, settings *protocol.WebcamStreamSettings) (frameSource, error) {
		return source, nil
	})

	startPayload := protocol.WebcamCommandPayload{Action: "start", SessionID: "session-42", DeviceID: "camera-x"}
	startData, _ := json.Marshal(startPayload)
	if result := manager.HandleCommand(context.Background(), Command{ID: "start", Payload: startData}); !result.Success {
		t.Fatalf("start failed: %s", result.Error)
	}

	waitForCondition(t, func() bool { return len(client.Requests()) >= 1 }, 2*time.Second)

	updatePayload := protocol.WebcamCommandPayload{
		Action:    "update",
		SessionID: "session-42",
		Settings:  &protocol.WebcamStreamSettings{Width: 1280, Height: 720, FrameRate: 30},
		Negotiation: &protocol.WebcamNegotiationState{
			Offer: &protocol.WebcamNegotiationOffer{Transport: "http"},
		},
	}
	updateData, _ := json.Marshal(updatePayload)
	updateResult := manager.HandleCommand(context.Background(), Command{ID: "update", Payload: updateData})
	if !updateResult.Success {
		t.Fatalf("update failed: %s", updateResult.Error)
	}

	waitForCondition(t, func() bool { return len(client.Requests()) >= 2 }, 2*time.Second)
	requests := client.Requests()
	patchReq := requests[len(requests)-1]
	if patchReq.Method != http.MethodPatch {
		t.Fatalf("expected PATCH request, got %s", patchReq.Method)
	}

	var patchBody map[string]any
	if err := json.Unmarshal(patchReq.Body, &patchBody); err != nil {
		t.Fatalf("failed to decode patch payload: %v", err)
	}
	if _, ok := patchBody["negotiation"]; !ok {
		t.Fatalf("expected negotiation payload: %+v", patchBody)
	}

	source.mu.Lock()
	if len(source.updates) != 1 {
		t.Fatalf("expected one update call, got %d", len(source.updates))
	}
	source.mu.Unlock()

	stopPayload := protocol.WebcamCommandPayload{Action: "stop", SessionID: "session-42"}
	stopData, _ := json.Marshal(stopPayload)
	_ = manager.HandleCommand(context.Background(), Command{ID: "stop", Payload: stopData})
	close(frames)
	waitForCondition(t, func() bool { manager.mu.Lock(); defer manager.mu.Unlock(); return len(manager.sessions) == 0 }, 2*time.Second)
}

func TestWebcamStreamStartFailurePropagates(t *testing.T) {
	manager := NewManager(Config{AgentID: "agent-2", BaseURL: "https://controller.example", Client: &fakeHTTPClient{}})
	manager.setFrameSourceFactory(func(deviceID string, settings *protocol.WebcamStreamSettings) (frameSource, error) {
		return nil, errors.New("no capture devices")
	})

	payload := protocol.WebcamCommandPayload{Action: "start", SessionID: "s-1", DeviceID: "camera-1"}
	data, _ := json.Marshal(payload)
	result := manager.HandleCommand(context.Background(), Command{ID: "cmd", Payload: data})
	if result.Success || result.Error == "" {
		t.Fatalf("expected failure, got %#v", result)
	}
}
