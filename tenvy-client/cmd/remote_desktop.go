package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/kbinani/screenshot"
	xdraw "golang.org/x/image/draw"
)

type RemoteDesktopQuality string

const (
	RemoteQualityAuto   RemoteDesktopQuality = "auto"
	RemoteQualityHigh   RemoteDesktopQuality = "high"
	RemoteQualityMedium RemoteDesktopQuality = "medium"
	RemoteQualityLow    RemoteDesktopQuality = "low"
)

type RemoteDesktopSettings struct {
	Quality  RemoteDesktopQuality `json:"quality"`
	Monitor  int                  `json:"monitor"`
	Mouse    bool                 `json:"mouse"`
	Keyboard bool                 `json:"keyboard"`
}

type RemoteDesktopSettingsPatch struct {
	Quality  *RemoteDesktopQuality `json:"quality,omitempty"`
	Monitor  *int                  `json:"monitor,omitempty"`
	Mouse    *bool                 `json:"mouse,omitempty"`
	Keyboard *bool                 `json:"keyboard,omitempty"`
}

type RemoteDesktopCommandPayload struct {
	Action    string                      `json:"action"`
	SessionID string                      `json:"sessionId,omitempty"`
	Settings  *RemoteDesktopSettingsPatch `json:"settings,omitempty"`
}

type RemoteDesktopFrameMetrics struct {
	FPS           float64 `json:"fps,omitempty"`
	BandwidthKbps float64 `json:"bandwidthKbps,omitempty"`
	CPUPercent    float64 `json:"cpuPercent,omitempty"`
	GPUPercent    float64 `json:"gpuPercent,omitempty"`
}

type RemoteDesktopMonitorInfo struct {
	ID     int    `json:"id"`
	Label  string `json:"label"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type RemoteDesktopDeltaRect struct {
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Encoding string `json:"encoding"`
	Data     string `json:"data"`
}

type RemoteDesktopFramePacket struct {
	SessionID string                     `json:"sessionId"`
	Sequence  uint64                     `json:"sequence"`
	Timestamp string                     `json:"timestamp"`
	Width     int                        `json:"width"`
	Height    int                        `json:"height"`
	KeyFrame  bool                       `json:"keyFrame"`
	Encoding  string                     `json:"encoding"`
	Image     string                     `json:"image,omitempty"`
	Deltas    []RemoteDesktopDeltaRect   `json:"deltas,omitempty"`
	Monitors  []RemoteDesktopMonitorInfo `json:"monitors,omitempty"`
	Metrics   *RemoteDesktopFrameMetrics `json:"metrics,omitempty"`
}

type RemoteDesktopSession struct {
	ID            string
	Settings      RemoteDesktopSettings
	Width         int
	Height        int
	TileSize      int
	FrameInterval time.Duration
	Sequence      uint64
	LastFrame     []byte
	ForceKeyFrame bool
	monitors      []remoteMonitor
	monitorInfos  []RemoteDesktopMonitorInfo
	monitorsDirty bool
	cancel        context.CancelFunc
}

type remoteMonitor struct {
	info   RemoteDesktopMonitorInfo
	bounds image.Rectangle
}

type RemoteDesktopStreamer struct {
	agent   *Agent
	mu      sync.Mutex
	session *RemoteDesktopSession
}

func NewRemoteDesktopStreamer(agent *Agent) *RemoteDesktopStreamer {
	return &RemoteDesktopStreamer{agent: agent}
}

func (s *RemoteDesktopStreamer) HandleCommand(ctx context.Context, cmd Command) CommandResult {
	var payload RemoteDesktopCommandPayload
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		return CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       fmt.Sprintf("invalid remote desktop payload: %v", err),
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	var err error
	switch strings.ToLower(strings.TrimSpace(payload.Action)) {
	case "start":
		err = s.startSession(ctx, payload)
	case "stop":
		err = s.stopSession(payload.SessionID)
	case "configure":
		err = s.configureSession(payload)
	default:
		err = fmt.Errorf("unsupported remote desktop action: %s", payload.Action)
	}

	result := CommandResult{
		CommandID:   cmd.ID,
		CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err != nil {
		result.Success = false
		result.Error = err.Error()
	} else {
		result.Success = true
		result.Output = fmt.Sprintf("remote desktop %s action processed", payload.Action)
	}
	return result
}

func (s *RemoteDesktopStreamer) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
}

func (s *RemoteDesktopStreamer) startSession(ctx context.Context, payload RemoteDesktopCommandPayload) error {
	sessionID := strings.TrimSpace(payload.SessionID)
	if sessionID == "" {
		return errors.New("missing session identifier")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session != nil && s.session.ID != sessionID {
		s.stopLocked()
	}

	if s.session != nil && s.session.ID == sessionID {
		s.applySettingsLocked(s.session, payload.Settings)
		return nil
	}

	settings := defaultRemoteDesktopSettings()
	applySettingsPatch(&settings, payload.Settings)

	monitors := detectRemoteMonitors()
	infos := monitorInfos(monitors)
	if len(infos) == 0 {
		infos = []RemoteDesktopMonitorInfo{{ID: 0, Label: "Primary", Width: 1280, Height: 720}}
		monitors = []remoteMonitor{{
			info:   infos[0],
			bounds: image.Rect(0, 0, infos[0].Width, infos[0].Height),
		}}
	}

	settings.Monitor = clampMonitorIndex(monitors, settings.Monitor)
	monitorInfo := infos[settings.Monitor]
	width, height, tile, interval := qualityProfile(settings.Quality, monitorInfo)

	streamCtx, cancel := context.WithCancel(context.Background())
	session := &RemoteDesktopSession{
		ID:            sessionID,
		Settings:      settings,
		Width:         width,
		Height:        height,
		TileSize:      tile,
		FrameInterval: interval,
		ForceKeyFrame: true,
		monitors:      monitors,
		monitorInfos:  infos,
		monitorsDirty: true,
		cancel:        cancel,
	}
	s.session = session

	go s.stream(streamCtx, session)
	s.agent.logger.Printf("remote desktop session %s started", sessionID)
	return nil
}

func (s *RemoteDesktopStreamer) stopSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session == nil {
		return nil
	}
	if strings.TrimSpace(sessionID) != "" && sessionID != s.session.ID {
		return fmt.Errorf("session %s not active", sessionID)
	}

	s.agent.logger.Printf("remote desktop session %s stopped", s.session.ID)
	s.stopLocked()
	return nil
}

func (s *RemoteDesktopStreamer) configureSession(payload RemoteDesktopCommandPayload) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session == nil {
		return errors.New("remote desktop session not active")
	}
	if strings.TrimSpace(payload.SessionID) != "" && payload.SessionID != s.session.ID {
		return fmt.Errorf("session %s not active", payload.SessionID)
	}

	s.applySettingsLocked(s.session, payload.Settings)
	return nil
}

func (s *RemoteDesktopStreamer) stopLocked() {
	if s.session != nil {
		if s.session.cancel != nil {
			s.session.cancel()
		}
		s.session = nil
	}
}

func (s *RemoteDesktopStreamer) applySettingsLocked(session *RemoteDesktopSession, patch *RemoteDesktopSettingsPatch) {
	if session == nil || patch == nil {
		return
	}

	prevMonitor := session.Settings.Monitor

	if patch.Quality != nil {
		session.Settings.Quality = normalizeQuality(*patch.Quality)
	}
	if patch.Monitor != nil {
		session.Settings.Monitor = *patch.Monitor
	}
	if patch.Mouse != nil {
		session.Settings.Mouse = *patch.Mouse
	}
	if patch.Keyboard != nil {
		session.Settings.Keyboard = *patch.Keyboard
	}

	if len(session.monitors) == 0 || len(session.monitorInfos) == 0 {
		s.refreshMonitorsLocked(session, false)
	}

	session.Settings.Monitor = clampMonitorIndex(session.monitors, session.Settings.Monitor)
	monitorInfo := session.monitorInfos[session.Settings.Monitor]

	width, height, tile, interval := qualityProfile(session.Settings.Quality, monitorInfo)
	if width != session.Width || height != session.Height {
		session.LastFrame = nil
		session.ForceKeyFrame = true
	}
	if session.Settings.Monitor != prevMonitor {
		session.LastFrame = nil
		session.ForceKeyFrame = true
	}
	session.Width = width
	session.Height = height
	session.TileSize = tile
	session.FrameInterval = interval
}

func (s *RemoteDesktopStreamer) stream(ctx context.Context, session *RemoteDesktopSession) {
	var lastSent time.Time

	for {
		s.mu.Lock()
		if s.session == nil || s.session.ID != session.ID {
			s.mu.Unlock()
			return
		}
		interval := session.FrameInterval
		s.mu.Unlock()

		if interval <= 0 {
			interval = 100 * time.Millisecond
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}

		s.mu.Lock()
		if s.session == nil || s.session.ID != session.ID {
			s.mu.Unlock()
			return
		}

		s.refreshMonitorsLocked(session, false)

		monitorIndex := clampMonitorIndex(session.monitors, session.Settings.Monitor)
		session.Settings.Monitor = monitorIndex

		var monitorsPayload []RemoteDesktopMonitorInfo
		if session.monitorsDirty {
			monitorsPayload = append([]RemoteDesktopMonitorInfo(nil), session.monitorInfos...)
		}

		monitor := session.monitors[monitorIndex]
		width := session.Width
		height := session.Height
		tile := session.TileSize
		interval = session.FrameInterval
		forceKey := session.ForceKeyFrame || len(session.LastFrame) == 0
		prev := append([]byte(nil), session.LastFrame...)
		sequence := session.Sequence + 1
		session.Sequence = sequence
		session.ForceKeyFrame = false
		s.mu.Unlock()

		processStart := time.Now()
		current, captureErr := captureMonitorFrame(monitor, width, height)
		if captureErr != nil {
			s.agent.logger.Printf("remote desktop capture error: %v", captureErr)
			s.mu.Lock()
			s.refreshMonitorsLocked(session, true)
			s.mu.Unlock()
			continue
		}

		keyFrame := forceKey || len(prev) != len(current) || len(prev) == 0
		var imageData string
		var deltas []RemoteDesktopDeltaRect
		bytesSent := 0

		if keyFrame {
			encoded, err := encodePNG(width, height, current)
			if err != nil {
				s.agent.logger.Printf("remote desktop encode frame: %v", err)
				continue
			}
			imageData = encoded
			bytesSent += len(encoded)
		} else {
			rects, err := diffFrames(prev, current, width, height, tile)
			if err != nil {
				s.agent.logger.Printf("remote desktop diff error: %v", err)
				keyFrame = true
				if encoded, encErr := encodePNG(width, height, current); encErr == nil {
					imageData = encoded
					bytesSent += len(encoded)
				} else {
					s.agent.logger.Printf("remote desktop fallback encode: %v", encErr)
					continue
				}
			} else {
				deltas = rects
				for _, rect := range rects {
					bytesSent += len(rect.Data)
				}
				if len(rects) == 0 {
					if time.Since(lastSent) > 3*interval {
						keyFrame = true
						if encoded, encErr := encodePNG(width, height, current); encErr == nil {
							imageData = encoded
							bytesSent += len(encoded)
						}
					}
				}
			}
		}

		processingDuration := time.Since(processStart)
		frameDuration := interval
		if !lastSent.IsZero() {
			elapsed := time.Since(lastSent)
			if elapsed > 0 {
				frameDuration = elapsed
			}
		}

		metrics := computeMetrics(interval, frameDuration, processingDuration, bytesSent)
		timestamp := time.Now()
		frame := RemoteDesktopFramePacket{
			SessionID: session.ID,
			Sequence:  sequence,
			Timestamp: timestamp.UTC().Format(time.RFC3339Nano),
			Width:     width,
			Height:    height,
			KeyFrame:  keyFrame,
			Encoding:  "png",
			Metrics:   metrics,
		}
		if len(monitorsPayload) > 0 {
			frame.Monitors = monitorsPayload
		}
		if keyFrame {
			frame.Image = imageData
			frame.Deltas = nil
		} else {
			frame.Deltas = deltas
		}

		sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := s.sendFrame(sendCtx, frame)
		cancel()
		if err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				s.agent.logger.Printf("remote desktop frame send error: %v", err)
			}
			continue
		}

		s.mu.Lock()
		if s.session != nil && s.session.ID == session.ID {
			s.session.LastFrame = current
			if len(monitorsPayload) > 0 {
				s.session.monitorsDirty = false
			}
		}
		s.mu.Unlock()

		lastSent = timestamp
	}
}

func (s *RemoteDesktopStreamer) sendFrame(ctx context.Context, frame RemoteDesktopFramePacket) error {
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/api/agents/%s/remote-desktop/frames", s.agent.baseURL, url.PathEscape(s.agent.id))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent())
	if strings.TrimSpace(s.agent.key) != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.agent.key))
	}

	resp, err := s.agent.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("frame upload failed: status %d", resp.StatusCode)
	}
	return nil
}

func defaultRemoteDesktopSettings() RemoteDesktopSettings {
	return RemoteDesktopSettings{
		Quality:  RemoteQualityAuto,
		Monitor:  0,
		Mouse:    true,
		Keyboard: true,
	}
}

func applySettingsPatch(settings *RemoteDesktopSettings, patch *RemoteDesktopSettingsPatch) {
	if patch == nil {
		return
	}
	if patch.Quality != nil {
		settings.Quality = normalizeQuality(*patch.Quality)
	}
	if patch.Monitor != nil && *patch.Monitor >= 0 {
		settings.Monitor = *patch.Monitor
	}
	if patch.Mouse != nil {
		settings.Mouse = *patch.Mouse
	}
	if patch.Keyboard != nil {
		settings.Keyboard = *patch.Keyboard
	}
}

func normalizeQuality(value RemoteDesktopQuality) RemoteDesktopQuality {
	switch strings.ToLower(string(value)) {
	case string(RemoteQualityHigh):
		return RemoteQualityHigh
	case string(RemoteQualityMedium):
		return RemoteQualityMedium
	case string(RemoteQualityLow):
		return RemoteQualityLow
	default:
		return RemoteQualityAuto
	}
}

func qualityProfile(quality RemoteDesktopQuality, monitor RemoteDesktopMonitorInfo) (int, int, int, time.Duration) {
	width := monitor.Width
	height := monitor.Height
	if width <= 0 {
		width = 1280
	}
	if height <= 0 {
		height = 720
	}

	var targetWidth int
	var interval time.Duration
	var tile int

	switch quality {
	case RemoteQualityHigh:
		targetWidth = minInt(width, 1920)
		interval = 80 * time.Millisecond
		tile = 32
	case RemoteQualityMedium:
		targetWidth = minInt(width, 1366)
		interval = 120 * time.Millisecond
		tile = 48
	case RemoteQualityLow:
		targetWidth = minInt(width, 1024)
		interval = 180 * time.Millisecond
		tile = 64
	default:
		targetWidth = width
		interval = 100 * time.Millisecond
		tile = 40
		if width >= 2560 {
			targetWidth = 1600
			interval = 110 * time.Millisecond
		} else if width > 1920 {
			targetWidth = 1760
			interval = 105 * time.Millisecond
		} else if width > 1366 {
			targetWidth = 1440
		}
	}

	if targetWidth <= 0 {
		targetWidth = width
	}

	scale := float64(targetWidth) / float64(width)
	if scale <= 0 {
		scale = 1
	}
	targetHeight := int(math.Round(float64(height) * scale))
	if targetHeight <= 0 {
		targetHeight = height
	}

	return targetWidth, targetHeight, tile, interval
}

func captureMonitorFrame(monitor remoteMonitor, width, height int) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, errors.New("invalid frame dimensions")
	}

	img, err := safeCaptureRect(monitor.bounds)
	if err != nil {
		return nil, err
	}
	if img.Bounds().Dx() == 0 || img.Bounds().Dy() == 0 {
		return nil, errors.New("empty monitor capture")
	}

	var rgba *image.RGBA
	if img.Bounds().Dx() != width || img.Bounds().Dy() != height {
		dst := image.NewRGBA(image.Rect(0, 0, width, height))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Over, nil)
		rgba = dst
	} else {
		rgba = img
	}

	return copyRGBA(rgba, width, height), nil
}

func copyRGBA(img *image.RGBA, width, height int) []byte {
	stride := img.Stride
	buf := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		start := y * width * 4
		src := img.Pix[y*stride : y*stride+width*4]
		copy(buf[start:start+width*4], src)
	}
	return buf
}

func safeCaptureRect(bounds image.Rectangle) (img *image.RGBA, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("capture panic: %v", r)
			img = nil
		}
	}()

	img, err = screenshot.CaptureRect(bounds)
	return img, err
}

func encodePNG(width, height int, data []byte) (string, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	copy(img.Pix, data)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func diffFrames(previous, current []byte, width, height, tile int) ([]RemoteDesktopDeltaRect, error) {
	if len(previous) != len(current) {
		return nil, errors.New("frame size mismatch")
	}
	if len(current) == 0 {
		return nil, nil
	}

	stride := width * 4
	var deltas []RemoteDesktopDeltaRect

	for y := 0; y < height; y += tile {
		h := minInt(tile, height-y)
		for x := 0; x < width; x += tile {
			w := minInt(tile, width-x)
			if regionChanged(previous, current, stride, x, y, w, h) {
				region := extractRegion(current, stride, x, y, w, h)
				encoded, err := encodePNG(w, h, region)
				if err != nil {
					return nil, err
				}
				deltas = append(deltas, RemoteDesktopDeltaRect{
					X:        x,
					Y:        y,
					Width:    w,
					Height:   h,
					Encoding: "png",
					Data:     encoded,
				})
			}
		}
	}

	return deltas, nil
}

func regionChanged(prev, curr []byte, stride, x, y, w, h int) bool {
	for row := 0; row < h; row++ {
		start := (y+row)*stride + x*4
		end := start + w*4
		if !bytes.Equal(prev[start:end], curr[start:end]) {
			return true
		}
	}
	return false
}

func extractRegion(data []byte, stride, x, y, w, h int) []byte {
	region := make([]byte, w*h*4)
	for row := 0; row < h; row++ {
		srcStart := (y+row)*stride + x*4
		copy(region[row*w*4:(row+1)*w*4], data[srcStart:srcStart+w*4])
	}
	return region
}

func (s *RemoteDesktopStreamer) refreshMonitorsLocked(session *RemoteDesktopSession, force bool) {
	monitors := detectRemoteMonitors()
	if len(monitors) == 0 {
		rect := image.Rect(0, 0, 1280, 720)
		monitors = []remoteMonitor{{
			info:   RemoteDesktopMonitorInfo{ID: 0, Label: "Primary", Width: rect.Dx(), Height: rect.Dy()},
			bounds: rect,
		}}
	}

	if !force && monitorsEquivalent(session.monitors, monitors) {
		if len(session.monitorInfos) == 0 {
			session.monitorInfos = monitorInfos(monitors)
		}
		return
	}

	session.monitors = monitors
	session.monitorInfos = monitorInfos(monitors)
	session.Settings.Monitor = clampMonitorIndex(monitors, session.Settings.Monitor)
	session.monitorsDirty = true
	session.LastFrame = nil
	session.ForceKeyFrame = true
}

func monitorInfos(monitors []remoteMonitor) []RemoteDesktopMonitorInfo {
	infos := make([]RemoteDesktopMonitorInfo, len(monitors))
	for i, monitor := range monitors {
		infos[i] = monitor.info
	}
	return infos
}

func detectRemoteMonitors() []remoteMonitor {
	count := safeNumDisplays()
	monitors := make([]remoteMonitor, 0, count)
	for i := 0; i < count; i++ {
		bounds, ok := safeGetDisplayBounds(i)
		if !ok {
			continue
		}
		width := bounds.Dx()
		height := bounds.Dy()
		if width <= 0 || height <= 0 {
			continue
		}
		info := RemoteDesktopMonitorInfo{
			ID:     i,
			Label:  fmt.Sprintf("Display %d", i+1),
			Width:  width,
			Height: height,
		}
		monitors = append(monitors, remoteMonitor{info: info, bounds: bounds})
	}
	return monitors
}

func safeNumDisplays() (count int) {
	defer func() {
		if r := recover(); r != nil {
			count = 0
		}
	}()
	count = screenshot.NumActiveDisplays()
	return
}

func safeGetDisplayBounds(index int) (rect image.Rectangle, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			rect = image.Rectangle{}
			ok = false
		}
	}()
	rect = screenshot.GetDisplayBounds(index)
	ok = true
	return
}

func monitorsEquivalent(a, b []remoteMonitor) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].info.ID != b[i].info.ID {
			return false
		}
		if a[i].info.Width != b[i].info.Width || a[i].info.Height != b[i].info.Height {
			return false
		}
	}
	return true
}

func clampMonitorIndex(monitors []remoteMonitor, index int) int {
	if len(monitors) == 0 {
		return 0
	}
	if index < 0 || index >= len(monitors) {
		return 0
	}
	return index
}

func computeMetrics(targetInterval, frameDuration, processing time.Duration, bytesSent int) *RemoteDesktopFrameMetrics {
	if targetInterval <= 0 {
		targetInterval = 100 * time.Millisecond
	}
	if frameDuration <= 0 {
		frameDuration = targetInterval
	}

	fps := 0.0
	if frameDuration > 0 {
		fps = 1.0 / frameDuration.Seconds()
	}
	bandwidth := 0.0
	if frameDuration > 0 {
		bandwidth = float64(bytesSent*8) / 1024 / frameDuration.Seconds()
	}

	cpuRatio := 0.0
	if targetInterval > 0 {
		cpuRatio = processing.Seconds() / targetInterval.Seconds()
	}
	if cpuRatio < 0 {
		cpuRatio = 0
	}
	cpuUsage := math.Min(95, math.Max(0, cpuRatio*100))
	gpuUsage := math.Min(90, cpuUsage*0.6)

	metrics := &RemoteDesktopFrameMetrics{
		FPS:           math.Round(fps*10) / 10,
		BandwidthKbps: math.Round(bandwidth*10) / 10,
	}

	if cpuUsage > 0 {
		metrics.CPUPercent = math.Round(cpuUsage*10) / 10
	}
	if gpuUsage > 0 {
		metrics.GPUPercent = math.Round(gpuUsage*10) / 10
	}

	return metrics
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
