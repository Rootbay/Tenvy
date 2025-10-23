//go:build freebsd

package webcam

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blackjack/webcam"
	"github.com/rootbay/tenvy-client/internal/protocol"
)

type fakeFileInfo struct {
	name string
	mode fs.FileMode
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeFileInfo) Sys() any           { return nil }

type fakeV4L2Device struct {
	mu       sync.Mutex
	waitCh   chan struct{}
	frames   [][]byte
	frameIdx int
	started  bool
	stopped  bool
	closed   bool
	format   webcam.PixelFormat
	width    uint32
	height   uint32
	fps      float32
	name     string
}

func newFakeV4L2Device(frames [][]byte) *fakeV4L2Device {
	return &fakeV4L2Device{
		waitCh: make(chan struct{}, 8),
		frames: frames,
		name:   "Fake FreeBSD Camera",
	}
}

func (f *fakeV4L2Device) StartStreaming() error {
	f.mu.Lock()
	f.started = true
	f.mu.Unlock()
	return nil
}

func (f *fakeV4L2Device) StopStreaming() error {
	f.mu.Lock()
	f.stopped = true
	f.mu.Unlock()
	return nil
}

func (f *fakeV4L2Device) Close() error {
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()
	return nil
}

func (f *fakeV4L2Device) WaitForFrame(timeout uint32) error {
	select {
	case <-f.waitCh:
		return nil
	case <-time.After(time.Duration(timeout) * time.Millisecond):
		return &webcam.Timeout{}
	}
}

func (f *fakeV4L2Device) ReadFrame() ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.frameIdx >= len(f.frames) {
		return nil, nil
	}
	data := append([]byte(nil), f.frames[f.frameIdx]...)
	f.frameIdx++
	return data, nil
}

func (f *fakeV4L2Device) SetImageFormat(format webcam.PixelFormat, width, height uint32) (webcam.PixelFormat, uint32, uint32, error) {
	f.mu.Lock()
	f.format = format
	f.width = width
	f.height = height
	f.mu.Unlock()
	return format, width, height, nil
}

func (f *fakeV4L2Device) SetFramerate(fps float32) error {
	f.mu.Lock()
	f.fps = fps
	f.mu.Unlock()
	return nil
}

func (f *fakeV4L2Device) GetSupportedFormats() map[webcam.PixelFormat]string {
	return map[webcam.PixelFormat]string{webcam.PixelFormat(1): "MJPG"}
}

func (f *fakeV4L2Device) GetSupportedFrameSizes(format webcam.PixelFormat) []webcam.FrameSize {
	return []webcam.FrameSize{{
		MinWidth:  640,
		MaxWidth:  640,
		MinHeight: 480,
		MaxHeight: 480,
	}}
}

func (f *fakeV4L2Device) GetSupportedFramerates(format webcam.PixelFormat, width, height uint32) []webcam.FrameRate {
	return []webcam.FrameRate{{
		MinNumerator:   1,
		MaxNumerator:   1,
		MinDenominator: 30,
		MaxDenominator: 30,
	}}
}

func (f *fakeV4L2Device) GetName() (string, error) {
	return f.name, nil
}

func (f *fakeV4L2Device) SignalFrame() {
	f.waitCh <- struct{}{}
}

func (f *fakeV4L2Device) WasStopped() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopped
}

func (f *fakeV4L2Device) WasClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

func TestFreeBSDInventoryCapturesWarnings(t *testing.T) {
	originalGlob := v4l2Glob
	originalStat := v4l2Stat
	originalOpen := openV4L2Device
	v4l2Glob = func(pattern string) ([]string, error) {
		return []string{"/dev/video0"}, nil
	}
	v4l2Stat = func(name string) (fs.FileInfo, error) {
		return fakeFileInfo{name: name, mode: fs.ModeDevice}, nil
	}
	openV4L2Device = func(path string) (v4l2Device, error) {
		return nil, errors.New("permission denied")
	}
	defer func() {
		v4l2Glob = originalGlob
		v4l2Stat = originalStat
		openV4L2Device = originalOpen
	}()

	client := &fakeHTTPClient{}
	manager := NewManager(Config{AgentID: "agent-bsd", BaseURL: "https://controller.example", Client: client})
	manager.setNowFunc(func() time.Time { return time.Date(2024, 7, 12, 8, 0, 0, 0, time.UTC) })

	payload := protocol.WebcamCommandPayload{Action: "enumerate", RequestID: "req-bsd"}
	data, _ := json.Marshal(payload)

	result := manager.HandleCommand(context.Background(), Command{ID: "cmd-enum", Payload: data})
	if !result.Success {
		t.Fatalf("enumeration failed: %s", result.Error)
	}

	waitForCondition(t, func() bool { return len(client.Requests()) == 1 }, 2*time.Second)
	reqs := client.Requests()
	if reqs[0].Method != http.MethodPost {
		t.Fatalf("expected POST, got %s", reqs[0].Method)
	}

	var body protocol.WebcamDeviceInventory
	if err := json.Unmarshal(reqs[0].Body, &body); err != nil {
		t.Fatalf("failed to decode inventory payload: %v", err)
	}
	if body.Warning == "" || body.Warning != "permission denied" && !strings.Contains(body.Warning, "permission denied") {
		t.Fatalf("expected warning to contain permission denied, got %q", body.Warning)
	}
}

func TestFreeBSDV4L2StreamLifecycle(t *testing.T) {
	fakeCam := newFakeV4L2Device([][]byte{[]byte("frame-payload")})
	originalOpen := openV4L2Device
	openV4L2Device = func(path string) (v4l2Device, error) {
		if path != "/dev/video0" {
			return nil, errors.New("unexpected device")
		}
		return fakeCam, nil
	}
	defer func() { openV4L2Device = originalOpen }()

	client := &fakeHTTPClient{}
	manager := NewManager(Config{AgentID: "agent-bsd", BaseURL: "https://controller.example", Client: client})
	manager.setNowFunc(func() time.Time { return time.Date(2024, 7, 12, 9, 0, 0, 0, time.UTC) })

	startPayload := protocol.WebcamCommandPayload{
		Action:    "start",
		SessionID: "session-bsd",
		DeviceID:  "/dev/video0",
		Settings:  &protocol.WebcamStreamSettings{FrameRate: 24},
	}
	startData, _ := json.Marshal(startPayload)
	result := manager.HandleCommand(context.Background(), Command{ID: "cmd-start", Payload: startData})
	if !result.Success {
		t.Fatalf("start failed: %s", result.Error)
	}

	waitForCondition(t, func() bool {
		for _, req := range client.Requests() {
			if req.Method == http.MethodPatch && strings.Contains(req.URL, "/webcam/sessions/session-bsd") {
				return true
			}
		}
		return false
	}, 5*time.Second)

	fakeCam.SignalFrame()

	waitForCondition(t, func() bool {
		for _, req := range client.Requests() {
			if req.Method == http.MethodPost && strings.Contains(req.URL, "/webcam/sessions/session-bsd/frames") {
				return true
			}
		}
		return false
	}, 5*time.Second)

	requests := client.Requests()
	var frameReq recordedRequest
	for _, req := range requests {
		if req.Method == http.MethodPost && strings.Contains(req.URL, "/frames") {
			frameReq = req
			break
		}
	}
	if frameReq.Method == "" {
		t.Fatalf("frame request not captured")
	}

	var frameBody map[string]string
	if err := json.Unmarshal(frameReq.Body, &frameBody); err != nil {
		t.Fatalf("failed to decode frame body: %v", err)
	}
	data, err := base64.StdEncoding.DecodeString(frameBody["data"])
	if err != nil {
		t.Fatalf("failed to decode frame payload: %v", err)
	}
	if string(data) != "frame-payload" {
		t.Fatalf("unexpected frame payload: %s", string(data))
	}

	stopPayload := protocol.WebcamCommandPayload{Action: "stop", SessionID: "session-bsd"}
	stopData, _ := json.Marshal(stopPayload)
	stopResult := manager.HandleCommand(context.Background(), Command{ID: "cmd-stop", Payload: stopData})
	if !stopResult.Success {
		t.Fatalf("stop failed: %s", stopResult.Error)
	}

	fakeCam.SignalFrame()

	waitForCondition(t, func() bool {
		for _, req := range client.Requests() {
			if req.Method == http.MethodPatch && strings.Contains(string(req.Body), "stopped") {
				return true
			}
		}
		return false
	}, 5*time.Second)

	if !fakeCam.WasStopped() {
		t.Fatalf("expected StopStreaming to be invoked")
	}
	if !fakeCam.WasClosed() {
		t.Fatalf("expected Close to be invoked")
	}
}
