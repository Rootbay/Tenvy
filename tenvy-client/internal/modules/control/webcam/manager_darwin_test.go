//go:build darwin

package webcam

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

func TestDarwinNativeStreamLifecycle(t *testing.T) {
	client := &fakeHTTPClient{}
	manager := NewManager(Config{AgentID: "agent-mac", BaseURL: "https://controller.example", Client: client})
	manager.setNowFunc(func() time.Time { return time.Date(2024, 7, 10, 12, 30, 0, 0, time.UTC) })

	payload := protocol.WebcamCommandPayload{
		Action:    "start",
		SessionID: "session-mac",
		DeviceID:  "camera-mac",
		Settings:  &protocol.WebcamStreamSettings{FrameRate: 24},
	}
	data, _ := json.Marshal(payload)

	result := manager.HandleCommand(context.Background(), Command{ID: "cmd-start-mac", Payload: data})
	if !result.Success {
		t.Fatalf("start failed: %s", result.Error)
	}

	waitForCondition(t, func() bool {
		for _, req := range client.Requests() {
			if req.Method == http.MethodPatch && strings.Contains(req.URL, "/webcam/sessions/session-mac") {
				return true
			}
		}
		return false
	}, 5*time.Second)

	stopPayload := protocol.WebcamCommandPayload{Action: "stop", SessionID: "session-mac"}
	stopData, _ := json.Marshal(stopPayload)

	stopResult := manager.HandleCommand(context.Background(), Command{ID: "cmd-stop-mac", Payload: stopData})
	if !stopResult.Success {
		t.Fatalf("stop failed: %s", stopResult.Error)
	}

	waitForCondition(t, func() bool {
		for _, req := range client.Requests() {
			if req.Method == http.MethodPatch && strings.Contains(string(req.Body), "stopped") {
				return true
			}
		}
		return false
	}, 5*time.Second)
}
