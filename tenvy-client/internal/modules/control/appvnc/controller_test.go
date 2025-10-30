package appvnc

import (
        "archive/zip"
        "bytes"
        "context"
        "encoding/base64"
        "encoding/json"
        "io"
        "net/http"
        "net/http/httptest"
        "os"
        "os/exec"
        "path/filepath"
        "strings"
        "testing"
        "time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

func TestHandleInputBurstValidatesSession(t *testing.T) {
	controller := NewController()

	err := controller.HandleInputBurst(context.Background(), protocol.AppVncInputBurst{
		SessionID: "session-1",
		Events: []protocol.AppVncInputEvent{{
			Type:       protocol.AppVncInputPointerMove,
			CapturedAt: 1,
			X:          0.1,
			Y:          0.2,
		}},
	})
	if err == nil || err.Error() != "no active session" {
		t.Fatalf("expected no active session error, got: %v", err)
	}

	controller.session = &sessionState{id: "session-1"}

	err = controller.HandleInputBurst(context.Background(), protocol.AppVncInputBurst{
		SessionID: " ",
		Events: []protocol.AppVncInputEvent{{
			Type:       protocol.AppVncInputPointerMove,
			CapturedAt: 2,
			X:          0.3,
			Y:          0.4,
		}},
	})
	if err == nil || err.Error() != "missing session identifier" {
		t.Fatalf("expected missing session identifier error, got: %v", err)
	}

	err = controller.HandleInputBurst(context.Background(), protocol.AppVncInputBurst{
		SessionID: "session-2",
		Events: []protocol.AppVncInputEvent{{
			Type:       protocol.AppVncInputPointerMove,
			CapturedAt: 3,
			X:          0.5,
			Y:          0.6,
		}},
	})
	if err == nil || err.Error() != "session identifier mismatch" {
		t.Fatalf("expected session identifier mismatch error, got: %v", err)
	}
}

func TestHandleInputBurstQueuesEvents(t *testing.T) {
	controller := NewController()
	controller.session = &sessionState{id: "session-1"}

	burst := protocol.AppVncInputBurst{
		SessionID: "session-1",
		Sequence:  7,
		Events: []protocol.AppVncInputEvent{{
			Type:       protocol.AppVncInputPointerButton,
			CapturedAt: 99,
			Button:     protocol.AppVncPointerButtonLeft,
			Pressed:    true,
		}},
	}

	if err := controller.HandleInputBurst(context.Background(), burst); err != nil {
		t.Fatalf("HandleInputBurst returned error: %v", err)
	}

	burst.Events[0].Pressed = false
	burst.Events[0].Button = protocol.AppVncPointerButtonRight

	controller.mu.Lock()
	defer controller.mu.Unlock()

	if controller.session == nil {
		t.Fatalf("session cleared unexpectedly")
	}
	if len(controller.session.inputQueue) != 1 {
		t.Fatalf("expected 1 queued burst, got %d", len(controller.session.inputQueue))
	}

	stored := controller.session.inputQueue[0]
	if stored.SessionID != "session-1" {
		t.Fatalf("unexpected stored session id: %s", stored.SessionID)
	}
	if stored.Sequence != 7 {
		t.Fatalf("unexpected stored sequence: %d", stored.Sequence)
	}
	if len(stored.Events) != 1 {
		t.Fatalf("unexpected stored event count: %d", len(stored.Events))
	}
	if !stored.Events[0].Pressed {
		t.Fatalf("expected stored event to remain pressed")
	}
	if stored.Events[0].Button != protocol.AppVncPointerButtonLeft {
		t.Fatalf("expected stored event button to remain left, got %s", stored.Events[0].Button)
	}
	if controller.session.lastSequence != 7 {
		t.Fatalf("unexpected last sequence: %d", controller.session.lastSequence)
	}
}

func TestCaptureLoopPostsFrames(t *testing.T) {
	controller := NewController()
	t.Cleanup(func() { controller.Shutdown(context.Background()) })

	frameCh := make(chan protocol.AppVncFramePacket, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/agents/test-agent/app-vnc/frames" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("unexpected authorization header: %s", got)
		}
		if got := r.Header.Get("User-Agent"); got != "TestAgent" {
			t.Errorf("unexpected user agent: %s", got)
		}
		defer r.Body.Close()

		var packet protocol.AppVncFramePacket
		if err := json.NewDecoder(r.Body).Decode(&packet); err != nil {
			t.Errorf("decode frame packet: %v", err)
		} else {
			select {
			case frameCh <- packet:
			default:
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accepted":true}`))
	}))
	t.Cleanup(server.Close)

	controller.Update(Config{
		WorkspaceRoot: t.TempDir(),
		AgentID:       "test-agent",
		BaseURL:       server.URL,
		AuthKey:       "secret",
		Client:        server.Client(),
		UserAgent:     "TestAgent",
	})
	controller.captureFactory = func(*sessionState) (surfaceCapturer, error) {
		return newFakeSurfaceCapturer(), nil
	}
	controller.frameInterval = func(protocol.AppVncQuality) time.Duration { return 10 * time.Millisecond }
	controller.requestTimeout = 500 * time.Millisecond
	controller.now = func() time.Time { return time.Unix(0, 0) }

	appID := "browser"
	window := "Test Window"
	payload := protocol.AppVncCommandPayload{
		SessionID: "session-1",
		Application: &protocol.AppVncApplicationDescriptor{
			Platforms: []protocol.AppVncPlatform{protocol.AppVncPlatformLinux},
			Executable: map[protocol.AppVncPlatform]string{
				protocol.AppVncPlatformLinux: "/bin/true",
			},
		},
		Settings: &protocol.AppVncSessionSettingsPatch{
			AppID:       &appID,
			WindowTitle: &window,
		},
	}

	if err := controller.start(context.Background(), payload); err != nil {
		t.Fatalf("start session: %v", err)
	}

	var packet protocol.AppVncFramePacket
	select {
	case packet = <-frameCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for frame")
	}

	if packet.SessionID != "session-1" {
		t.Fatalf("unexpected session id: %s", packet.SessionID)
	}
	if packet.Sequence <= 0 {
		t.Fatalf("unexpected sequence: %d", packet.Sequence)
	}
	if packet.Width != 2 || packet.Height != 2 {
		t.Fatalf("unexpected dimensions: %dx%d", packet.Width, packet.Height)
	}
	if packet.Encoding != "jpeg" && packet.Encoding != "png" {
		t.Fatalf("unexpected encoding: %s", packet.Encoding)
	}
	if packet.Cursor == nil || !packet.Cursor.Visible {
		t.Fatalf("expected cursor metadata, got: %+v", packet.Cursor)
	}
	if packet.Metadata == nil {
		t.Fatalf("expected metadata")
	}
	if packet.Metadata.AppID != "browser" {
		t.Fatalf("unexpected metadata app id: %s", packet.Metadata.AppID)
	}
	if packet.Metadata.WindowTitle != "Test Window" {
		t.Fatalf("unexpected metadata window title: %s", packet.Metadata.WindowTitle)
	}
	if packet.Metadata.ProcessID == 0 {
		t.Fatalf("expected process id metadata")
	}
	if packet.Timestamp == "" {
		t.Fatalf("expected timestamp")
	}
	if _, err := time.Parse(time.RFC3339Nano, packet.Timestamp); err != nil {
		t.Fatalf("unexpected timestamp format: %v", err)
	}
	if len(packet.Image) == 0 {
		t.Fatalf("expected image data")
	}
	if _, err := base64.StdEncoding.DecodeString(packet.Image); err != nil {
		t.Fatalf("decode image payload: %v", err)
	}

	if err := controller.stop("session-1"); err != nil {
		t.Fatalf("stop session: %v", err)
	}
}

func TestStartDownloadsSeedBundles(t *testing.T) {
        profileBundle := createSeedArchive(t, map[string]string{
                "prefs.js": "{}",
        })
        dataBundle := createSeedArchive(t, map[string]string{
                "cache.db": "cache",
        })

	requestCounts := map[string]int{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCounts[r.URL.Path]++
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("unexpected authorization header: %s", got)
		}
		if got := r.Header.Get("X-Tenvy-Agent-ID"); got != "agent-zip" {
			t.Errorf("unexpected agent header: %s", got)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/app-vnc/seeds/profile/download"):
			if r.URL.Query().Get("agent") != "agent-zip" {
				t.Fatalf("unexpected agent query: %s", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(profileBundle)
		case strings.HasPrefix(r.URL.Path, "/api/app-vnc/seeds/data/download"):
			if r.URL.Query().Get("agent") != "agent-zip" {
				t.Fatalf("unexpected agent query: %s", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(dataBundle)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

        controller := NewController()
        controller.processWaiter = func(_ *Controller, cmd *exec.Cmd, _ string) {
                if cmd != nil {
                        _ = cmd.Wait()
                }
        }
        controller.Update(Config{
                WorkspaceRoot: t.TempDir(),
                AgentID:       "agent-zip",
		BaseURL:       server.URL,
		AuthKey:       "secret",
		Client:        server.Client(),
		UserAgent:     "TestAgent",
	})

	payload := protocol.AppVncCommandPayload{
		SessionID: "seed-session",
		Application: &protocol.AppVncApplicationDescriptor{
			Platforms: []protocol.AppVncPlatform{protocol.AppVncPlatformLinux},
			Executable: map[protocol.AppVncPlatform]string{
				protocol.AppVncPlatformLinux: "/bin/true",
			},
		},
		Virtualization: &protocol.AppVncVirtualizationPlan{
			ProfileSeed: "/api/app-vnc/seeds/profile/download?agent=agent-zip",
			DataRoot:    "/api/app-vnc/seeds/data/download?agent=agent-zip",
		},
	}

	if err := controller.start(context.Background(), payload); err != nil {
		t.Fatalf("start session: %v", err)
	}
	t.Cleanup(func() { controller.Shutdown(context.Background()) })

	controller.mu.Lock()
	workspace := ""
	if controller.session != nil {
		workspace = controller.session.workspace
	}
	controller.mu.Unlock()
	if workspace == "" {
		t.Fatal("workspace not initialized")
	}

	if _, err := os.Stat(filepath.Join(workspace, "profile", "prefs.js")); err != nil {
		t.Fatalf("profile seed not extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "data", "cache.db")); err != nil {
		t.Fatalf("data seed not extracted: %v", err)
	}

	if count := requestCounts["/api/app-vnc/seeds/profile/download"]; count == 0 {
		t.Fatalf("profile bundle not requested")
	}
	if count := requestCounts["/api/app-vnc/seeds/data/download"]; count == 0 {
		t.Fatalf("data bundle not requested")
	}
}

func createSeedArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for name, content := range files {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := io.WriteString(fw, content); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buf.Bytes()
}

type fakeSurfaceCapturer struct {
	frame *surfaceFrame
}

func newFakeSurfaceCapturer() *fakeSurfaceCapturer {
	data := []byte{
		0x00, 0x01, 0x02, 0x03,
		0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b,
		0x0c, 0x0d, 0x0e, 0x0f,
	}
	return &fakeSurfaceCapturer{
		frame: &surfaceFrame{
			image: &surfaceImage{
				width:  2,
				height: 2,
				stride: 8,
				data:   data,
			},
			cursor: &protocol.AppVncCursorState{X: 0.5, Y: 0.5, Visible: true},
		},
	}
}

func (f *fakeSurfaceCapturer) Capture(ctx context.Context) (*surfaceFrame, error) {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}
	copyData := append([]byte(nil), f.frame.image.data...)
	return &surfaceFrame{
		image: &surfaceImage{
			width:  f.frame.image.width,
			height: f.frame.image.height,
			stride: f.frame.image.stride,
			data:   copyData,
		},
		cursor: f.frame.cursor,
	}, nil
}

func (f *fakeSurfaceCapturer) Close() error { return nil }
