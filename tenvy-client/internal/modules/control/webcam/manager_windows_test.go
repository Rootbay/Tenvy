//go:build windows

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

func TestWindowsNativeStreamLifecycle(t *testing.T) {
	client := &fakeHTTPClient{}
	manager := NewManager(Config{AgentID: "agent-win", BaseURL: "https://controller.example", Client: client})
	manager.setNowFunc(func() time.Time { return time.Date(2024, 7, 10, 12, 0, 0, 0, time.UTC) })

	payload := protocol.WebcamCommandPayload{
		Action:    "start",
		SessionID: "session-win",
		DeviceID:  "camera-win",
		Settings:  &protocol.WebcamStreamSettings{FrameRate: 15},
	}
	data, _ := json.Marshal(payload)

	result := manager.HandleCommand(context.Background(), Command{ID: "cmd-start-win", Payload: data})
	if !result.Success {
		t.Fatalf("start failed: %s", result.Error)
	}

	waitForCondition(t, func() bool {
		for _, req := range client.Requests() {
			if req.Method == http.MethodPatch && strings.Contains(req.URL, "/webcam/sessions/session-win") {
				return true
			}
		}
		return false
	}, 5*time.Second)

	stopPayload := protocol.WebcamCommandPayload{Action: "stop", SessionID: "session-win"}
	stopData, _ := json.Marshal(stopPayload)

	stopResult := manager.HandleCommand(context.Background(), Command{ID: "cmd-stop-win", Payload: stopData})
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
