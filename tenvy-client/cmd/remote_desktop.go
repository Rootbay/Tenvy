package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/kbinani/screenshot"
	xdraw "golang.org/x/image/draw"
)

type RemoteDesktopQuality string

type RemoteDesktopStreamMode string

const (
	RemoteQualityAuto   RemoteDesktopQuality = "auto"
	RemoteQualityHigh   RemoteDesktopQuality = "high"
	RemoteQualityMedium RemoteDesktopQuality = "medium"
	RemoteQualityLow    RemoteDesktopQuality = "low"

	RemoteStreamModeImages RemoteDesktopStreamMode = "images"
	RemoteStreamModeVideo  RemoteDesktopStreamMode = "video"
)

type RemoteDesktopInputType string

const (
	RemoteInputMouseMove   RemoteDesktopInputType = "mouse-move"
	RemoteInputMouseButton RemoteDesktopInputType = "mouse-button"
	RemoteInputMouseScroll RemoteDesktopInputType = "mouse-scroll"
	RemoteInputKey         RemoteDesktopInputType = "key"
)

type RemoteDesktopMouseButton string

const (
	RemoteMouseButtonLeft   RemoteDesktopMouseButton = "left"
	RemoteMouseButtonMiddle RemoteDesktopMouseButton = "middle"
	RemoteMouseButtonRight  RemoteDesktopMouseButton = "right"
)

type RemoteDesktopSettings struct {
	Quality  RemoteDesktopQuality    `json:"quality"`
	Monitor  int                     `json:"monitor"`
	Mouse    bool                    `json:"mouse"`
	Keyboard bool                    `json:"keyboard"`
	Mode     RemoteDesktopStreamMode `json:"mode"`
}

type RemoteDesktopSettingsPatch struct {
	Quality  *RemoteDesktopQuality    `json:"quality,omitempty"`
	Monitor  *int                     `json:"monitor,omitempty"`
	Mouse    *bool                    `json:"mouse,omitempty"`
	Keyboard *bool                    `json:"keyboard,omitempty"`
	Mode     *RemoteDesktopStreamMode `json:"mode,omitempty"`
}

type RemoteDesktopCommandPayload struct {
	Action    string                      `json:"action"`
	SessionID string                      `json:"sessionId,omitempty"`
	Settings  *RemoteDesktopSettingsPatch `json:"settings,omitempty"`
	Events    []RemoteDesktopInputEvent   `json:"events,omitempty"`
}

type RemoteDesktopInputEvent struct {
	Type       RemoteDesktopInputType   `json:"type"`
	X          float64                  `json:"x,omitempty"`
	Y          float64                  `json:"y,omitempty"`
	Normalized bool                     `json:"normalized,omitempty"`
	Monitor    *int                     `json:"monitor,omitempty"`
	Button     RemoteDesktopMouseButton `json:"button,omitempty"`
	Pressed    bool                     `json:"pressed,omitempty"`
	DeltaX     float64                  `json:"deltaX,omitempty"`
	DeltaY     float64                  `json:"deltaY,omitempty"`
	DeltaMode  int                      `json:"deltaMode,omitempty"`
	Key        string                   `json:"key,omitempty"`
	Code       string                   `json:"code,omitempty"`
	KeyCode    int                      `json:"keyCode,omitempty"`
	Repeat     bool                     `json:"repeat,omitempty"`
	AltKey     bool                     `json:"altKey,omitempty"`
	CtrlKey    bool                     `json:"ctrlKey,omitempty"`
	ShiftKey   bool                     `json:"shiftKey,omitempty"`
	MetaKey    bool                     `json:"metaKey,omitempty"`
}

type RemoteDesktopFrameMetrics struct {
	FPS           float64 `json:"fps,omitempty"`
	BandwidthKbps float64 `json:"bandwidthKbps,omitempty"`
	CPUPercent    float64 `json:"cpuPercent,omitempty"`
	GPUPercent    float64 `json:"gpuPercent,omitempty"`
	ClipQuality   int     `json:"clipQuality,omitempty"`
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
	Clip      *RemoteDesktopVideoClip    `json:"clip,omitempty"`
	Monitors  []RemoteDesktopMonitorInfo `json:"monitors,omitempty"`
	Metrics   *RemoteDesktopFrameMetrics `json:"metrics,omitempty"`
}

type RemoteDesktopVideoClip struct {
	DurationMs int                      `json:"durationMs"`
	Frames     []RemoteDesktopClipFrame `json:"frames"`
}

type RemoteDesktopClipFrame struct {
	OffsetMs int    `json:"offsetMs"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Encoding string `json:"encoding"`
	Data     string `json:"data"`
}

type RemoteDesktopSession struct {
	ID                 string
	Settings           RemoteDesktopSettings
	Width              int
	Height             int
	TileSize           int
	ClipQuality        int
	FrameInterval      time.Duration
	Sequence           uint64
	LastFrame          []byte
	ForceKeyFrame      bool
	BaseWidth          int
	BaseHeight         int
	NativeWidth        int
	NativeHeight       int
	AdaptiveScale      float64
	MinScale           float64
	MaxScale           float64
	BaseInterval       time.Duration
	MinInterval        time.Duration
	MaxInterval        time.Duration
	BaseTile           int
	MinTile            int
	MaxTile            int
	MinClipQuality     int
	MaxClipQuality     int
	LastAdaptation     time.Time
	monitors           []remoteMonitor
	monitorInfos       []RemoteDesktopMonitorInfo
	monitorsDirty      bool
	lastMonitorRefresh time.Time
	cancel             context.CancelFunc
}

type remoteMonitor struct {
	info   RemoteDesktopMonitorInfo
	bounds image.Rectangle
}

type RemoteDesktopStreamer struct {
	controller *remoteDesktopSessionController
}

type remoteDesktopSessionController struct {
	agent   *Agent
	mu      sync.Mutex
	session *RemoteDesktopSession
}

type remoteTileHasher struct {
	tile      int
	cols      int
	rows      int
	width     int
	height    int
	ready     bool
	checksums []uint64
}

func (h *remoteTileHasher) reset() {
	h.tile = 0
	h.cols = 0
	h.rows = 0
	h.width = 0
	h.height = 0
	h.ready = false
	h.checksums = h.checksums[:0]
}

func (h *remoteTileHasher) ensure(width, height, tile int) {
	if tile <= 0 {
		tile = 32
	}

	cols := (width + tile - 1) / tile
	rows := (height + tile - 1) / tile
	total := cols * rows

	if h.tile != tile || h.cols != cols || h.rows != rows || h.width != width || h.height != height || len(h.checksums) != total {
		if cap(h.checksums) < total {
			h.checksums = make([]uint64, total)
		} else {
			h.checksums = h.checksums[:total]
			for i := range h.checksums {
				h.checksums[i] = 0
			}
		}
		h.tile = tile
		h.cols = cols
		h.rows = rows
		h.width = width
		h.height = height
		h.ready = false
	}
}

func (h *remoteTileHasher) rebuild(data []byte, width, height, tile int) {
	if data == nil || width <= 0 || height <= 0 {
		h.reset()
		return
	}

	h.ensure(width, height, tile)
	stride := width * 4
	idx := 0
	for y := 0; y < height; y += h.tile {
		hgt := minInt(h.tile, height-y)
		for x := 0; x < width; x += h.tile {
			wdt := minInt(h.tile, width-x)
			h.checksums[idx] = tileChecksum(data, stride, x, y, wdt, hgt)
			idx++
		}
	}
	h.ready = true
}

func (c *remoteDesktopSessionController) Shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopLocked()
}

const (
	remoteEncodingPNG      = "png"
	remoteEncodingJPEG     = "jpeg"
	remoteEncodingClip     = "clip"
	remoteClipEncodingJPEG = "jpeg"
	minClipDuration        = 120 * time.Millisecond
	maxClipDuration        = 350 * time.Millisecond
	defaultClipDuration    = 220 * time.Millisecond
	maxClipFrames          = 12
	minClipQuality         = 45
	maxClipQuality         = 92
	defaultClipQuality     = 80
	clipQualityStepDown    = 6
	clipQualityStepUp      = 3
)

const monitorRefreshInterval = 3 * time.Second

var (
	pngEncoder      = png.Encoder{CompressionLevel: png.BestSpeed}
	imageBufferPool = sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}
	frameBufferPool = sync.Pool{New: func() interface{} { return make([]byte, 0) }}
	jpegOptionsPool = sync.Pool{New: func() interface{} { return new(jpeg.Options) }}
)

const (
	maxDeltaCoverageRatio = 0.35
	maxDeltaTileFactor    = 3
	jpegKeyFrameMinPixels = 320 * 240
	jpegDeltaMinPixels    = 32 * 32
)

func NewRemoteDesktopStreamer(agent *Agent) *RemoteDesktopStreamer {
	return &RemoteDesktopStreamer{
		controller: newRemoteDesktopSessionController(agent),
	}
}

func newRemoteDesktopSessionController(agent *Agent) *remoteDesktopSessionController {
	return &remoteDesktopSessionController{agent: agent}
}

func (s *RemoteDesktopStreamer) HandleCommand(ctx context.Context, cmd Command) CommandResult {
	payload, err := decodeRemoteDesktopPayload(cmd.Payload)
	if err != nil {
		return CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       err.Error(),
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	var actionErr error
	switch strings.ToLower(strings.TrimSpace(payload.Action)) {
	case "start":
		actionErr = s.controller.Start(ctx, payload)
	case "stop":
		actionErr = s.controller.Stop(payload.SessionID)
	case "configure":
		actionErr = s.controller.Configure(payload)
	case "input":
		actionErr = s.controller.HandleInput(payload)
	default:
		actionErr = fmt.Errorf("unsupported remote desktop action: %s", payload.Action)
	}

	result := CommandResult{
		CommandID:   cmd.ID,
		CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if actionErr != nil {
		result.Success = false
		result.Error = actionErr.Error()
	} else {
		result.Success = true
		result.Output = fmt.Sprintf("remote desktop %s action processed", payload.Action)
	}
	return result
}

func (s *RemoteDesktopStreamer) Shutdown() {
	s.controller.Shutdown()
}

func decodeRemoteDesktopPayload(raw json.RawMessage) (RemoteDesktopCommandPayload, error) {
	var payload RemoteDesktopCommandPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return RemoteDesktopCommandPayload{}, fmt.Errorf("invalid remote desktop payload: %w", err)
	}
	return payload, nil
}

func (c *remoteDesktopSessionController) Start(ctx context.Context, payload RemoteDesktopCommandPayload) error {
	sessionID := strings.TrimSpace(payload.SessionID)
	if sessionID == "" {
		return errors.New("missing session identifier")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session != nil && c.session.ID != sessionID {
		c.stopLocked()
	}

	if c.session != nil && c.session.ID == sessionID {
		c.applySettingsLocked(c.session, payload.Settings)
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
		ForceKeyFrame: true,
		monitors:      monitors,
		monitorInfos:  infos,
		monitorsDirty: true,
		cancel:        cancel,
	}
	c.configureProfileLocked(session, monitorInfo, width, height, tile, interval, true)
	c.session = session

	go c.stream(streamCtx, session)
	c.agent.logger.Printf("remote desktop session %s started", sessionID)
	return nil
}

func (c *remoteDesktopSessionController) Stop(sessionID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session == nil {
		return nil
	}
	if strings.TrimSpace(sessionID) != "" && sessionID != c.session.ID {
		return fmt.Errorf("session %s not active", sessionID)
	}

	c.agent.logger.Printf("remote desktop session %s stopped", c.session.ID)
	c.stopLocked()
	return nil
}

func (c *remoteDesktopSessionController) Configure(payload RemoteDesktopCommandPayload) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session == nil {
		return errors.New("remote desktop session not active")
	}
	if strings.TrimSpace(payload.SessionID) != "" && payload.SessionID != c.session.ID {
		return fmt.Errorf("session %s not active", payload.SessionID)
	}

	c.applySettingsLocked(c.session, payload.Settings)
	return nil
}

func (c *remoteDesktopSessionController) HandleInput(payload RemoteDesktopCommandPayload) error {
	if len(payload.Events) == 0 {
		return nil
	}

	sessionID := strings.TrimSpace(payload.SessionID)

	c.mu.Lock()
	if c.session == nil {
		c.mu.Unlock()
		return errors.New("remote desktop session not active")
	}
	if sessionID != "" && sessionID != c.session.ID {
		c.mu.Unlock()
		return fmt.Errorf("session %s not active", sessionID)
	}

	settings := c.session.Settings
	monitors := append([]remoteMonitor(nil), c.session.monitors...)
	c.mu.Unlock()

	filtered := make([]RemoteDesktopInputEvent, 0, len(payload.Events))
	for _, event := range payload.Events {
		switch event.Type {
		case RemoteInputMouseMove, RemoteInputMouseButton, RemoteInputMouseScroll:
			if !settings.Mouse {
				continue
			}
		case RemoteInputKey:
			if !settings.Keyboard {
				continue
			}
		default:
			continue
		}
		filtered = append(filtered, event)
	}

	if len(filtered) == 0 {
		return nil
	}

	return processRemoteInput(monitors, settings, filtered)
}

func (c *remoteDesktopSessionController) stopLocked() {
	if c.session != nil {
		if c.session.cancel != nil {
			c.session.cancel()
		}
		c.session = nil
	}
}

func (c *remoteDesktopSessionController) applySettingsLocked(session *RemoteDesktopSession, patch *RemoteDesktopSettingsPatch) {
	if session == nil || patch == nil {
		return
	}

	prevMonitor := session.Settings.Monitor
	prevMode := session.Settings.Mode

	if patch.Quality != nil {
		session.Settings.Quality = normalizeQuality(*patch.Quality)
		session.AdaptiveScale = 1
		session.LastAdaptation = time.Time{}
		session.ClipQuality = 0
	}
	if patch.Mode != nil {
		session.Settings.Mode = normalizeStreamMode(*patch.Mode)
		if session.Settings.Mode != prevMode {
			session.ForceKeyFrame = true
			releaseFrameBuffer(session.LastFrame)
			session.LastFrame = nil
			session.ClipQuality = 0
		}
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
		c.refreshMonitorsLocked(session, false)
	}

	session.Settings.Monitor = clampMonitorIndex(session.monitors, session.Settings.Monitor)
	monitorInfo := session.monitorInfos[session.Settings.Monitor]

	width, height, tile, interval := qualityProfile(session.Settings.Quality, monitorInfo)
	forceKey := session.Settings.Monitor != prevMonitor
	c.configureProfileLocked(session, monitorInfo, width, height, tile, interval, forceKey)
}

func (c *remoteDesktopSessionController) configureProfileLocked(
	session *RemoteDesktopSession,
	monitor RemoteDesktopMonitorInfo,
	width, height, tile int,
	interval time.Duration,
	forceKey bool,
) {
	if session == nil {
		return
	}

	if session.Settings.Mode == "" {
		session.Settings.Mode = RemoteStreamModeVideo
	}

	baseWidth := maxInt(1, width)
	baseHeight := maxInt(1, height)
	session.BaseWidth = baseWidth
	session.BaseHeight = baseHeight

	nativeWidth := monitor.Width
	if nativeWidth <= 0 {
		nativeWidth = baseWidth
	}
	nativeHeight := monitor.Height
	if nativeHeight <= 0 {
		nativeHeight = baseHeight
	}
	session.NativeWidth = nativeWidth
	session.NativeHeight = nativeHeight

	baseTile := tile
	if baseTile <= 0 {
		baseTile = 40
	}
	session.BaseTile = baseTile
	session.MinTile = maxInt(24, baseTile-16)
	session.MaxTile = minInt(120, baseTile+32)
	session.TileSize = clampInt(baseTile, session.MinTile, session.MaxTile)

	session.MinClipQuality = minClipQuality
	session.MaxClipQuality = maxClipQuality
	baseClipQuality := clipQualityBaseline(session.Settings.Quality)
	if baseClipQuality <= 0 {
		baseClipQuality = defaultClipQuality
	}
	baseClipQuality = clampInt(baseClipQuality, session.MinClipQuality, session.MaxClipQuality)
	if session.Settings.Mode != RemoteStreamModeVideo {
		session.ClipQuality = baseClipQuality
	} else {
		if session.ClipQuality == 0 || forceKey {
			session.ClipQuality = baseClipQuality
		} else {
			session.ClipQuality = clampInt(session.ClipQuality, session.MinClipQuality, session.MaxClipQuality)
		}
	}

	baseInterval := interval
	if baseInterval <= 0 {
		baseInterval = 100 * time.Millisecond
	}
	session.BaseInterval = baseInterval
	session.MinInterval = maxDuration(50*time.Millisecond, baseInterval/2)
	session.MaxInterval = minDuration(400*time.Millisecond, baseInterval*2)
	session.FrameInterval = clampDuration(baseInterval, session.MinInterval, session.MaxInterval)

	resolutionChanged := false

	if session.Settings.Quality == RemoteQualityAuto {
		if session.AdaptiveScale <= 0 {
			session.AdaptiveScale = 1
		}
		session.MinScale = 0.5
		maxScale := float64(session.NativeWidth) / float64(session.BaseWidth)
		if maxScale < 1 {
			maxScale = 1
		}
		session.MaxScale = math.Min(1.3, maxScale)
		if session.MaxScale < session.MinScale {
			session.MaxScale = session.MinScale
		}
		session.AdaptiveScale = clampFloat(session.AdaptiveScale, session.MinScale, session.MaxScale)
		if c.applyAdaptiveScaleLocked(session, forceKey) {
			resolutionChanged = true
		}
	} else {
		session.MinScale = 1
		session.MaxScale = 1
		session.AdaptiveScale = 1
		if session.Width != session.BaseWidth || session.Height != session.BaseHeight {
			session.Width = session.BaseWidth
			session.Height = session.BaseHeight
			resolutionChanged = true
		}
	}

	if session.Width == 0 || session.Height == 0 {
		session.Width = session.BaseWidth
		session.Height = session.BaseHeight
	}

	if session.Settings.Quality == RemoteQualityAuto {
		if session.TileSize == 0 {
			session.TileSize = clampInt(session.BaseTile, session.MinTile, session.MaxTile)
		} else {
			session.TileSize = clampInt(session.TileSize, session.MinTile, session.MaxTile)
		}
	} else {
		session.TileSize = clampInt(session.BaseTile, session.MinTile, session.MaxTile)
	}

	session.FrameInterval = clampDuration(session.FrameInterval, session.MinInterval, session.MaxInterval)

	if resolutionChanged {
		forceKey = true
	}

	if forceKey {
		session.LastFrame = nil
		session.ForceKeyFrame = true
		session.LastAdaptation = time.Time{}
	}
}

func (c *remoteDesktopSessionController) applyAdaptiveScaleLocked(session *RemoteDesktopSession, markKeyFrame bool) bool {
	if session == nil || session.Settings.Quality != RemoteQualityAuto {
		return false
	}

	scale := clampFloat(session.AdaptiveScale, session.MinScale, session.MaxScale)
	if scale <= 0 {
		scale = 1
	}
	width := clampInt(int(math.Round(float64(session.BaseWidth)*scale)), int(math.Round(float64(session.BaseWidth)*session.MinScale)), session.NativeWidth)
	height := clampInt(int(math.Round(float64(session.BaseHeight)*scale)), int(math.Round(float64(session.BaseHeight)*session.MinScale)), session.NativeHeight)
	if width <= 0 {
		width = session.BaseWidth
	}
	if height <= 0 {
		height = session.BaseHeight
	}

	if width == session.Width && height == session.Height {
		return false
	}

	session.Width = width
	session.Height = height
	session.LastFrame = nil
	if markKeyFrame {
		session.ForceKeyFrame = true
	}
	return true
}

func (c *remoteDesktopSessionController) maybeAdaptQualityLocked(
	session *RemoteDesktopSession,
	metrics *RemoteDesktopFrameMetrics,
	processing, frameDuration time.Duration,
	bytesSent int,
) {
	if session == nil || session.Settings.Quality != RemoteQualityAuto {
		return
	}

	now := time.Now()
	if !session.LastAdaptation.IsZero() && now.Sub(session.LastAdaptation) < 2*time.Second {
		return
	}

	var fps float64
	var bandwidth float64
	if metrics != nil {
		fps = metrics.FPS
		bandwidth = metrics.BandwidthKbps
	}
	if fps <= 0 && session.FrameInterval > 0 {
		fps = 1.0 / session.FrameInterval.Seconds()
	}
	if bandwidth <= 0 {
		if frameDuration > 0 {
			bandwidth = float64(bytesSent*8) / 1024 / frameDuration.Seconds()
		} else if session.FrameInterval > 0 {
			bandwidth = float64(bytesSent*8) / 1024 / session.FrameInterval.Seconds()
		}
	}

	slowThreshold := session.FrameInterval + session.FrameInterval/2
	fastThreshold := session.FrameInterval * 3 / 4

	lowFps := fps > 0 && fps < 8
	highBandwidth := bandwidth > 6000
	slowProcessing := processing > slowThreshold

	degrade := lowFps || highBandwidth || slowProcessing
	improve := false
	if fps > 18 && processing < slowThreshold {
		if bandwidth <= 0 || bandwidth < 2500 {
			improve = true
		}
	}
	if processing < fastThreshold && fps >= 20 {
		if bandwidth <= 0 || bandwidth < 3500 {
			improve = true
		}
	}

	if degrade && improve {
		improve = false
	}

	adapted := false
	clipAdjusted := false
	clipAdjustedDegrade := false

	if session.Settings.Mode == RemoteStreamModeVideo {
		if degrade {
			nextQuality := clampInt(session.ClipQuality-clipQualityStepDown, session.MinClipQuality, session.MaxClipQuality)
			if nextQuality != session.ClipQuality {
				session.ClipQuality = nextQuality
				clipAdjusted = true
				clipAdjustedDegrade = true
			}
		} else if improve {
			nextQuality := clampInt(session.ClipQuality+clipQualityStepUp, session.MinClipQuality, session.MaxClipQuality)
			if nextQuality != session.ClipQuality {
				session.ClipQuality = nextQuality
				clipAdjusted = true
			}
		}
	}

	if degrade {
		if slowProcessing || lowFps || !clipAdjustedDegrade {
			newScale := clampFloat(session.AdaptiveScale*0.85, session.MinScale, session.MaxScale)
			if newScale != session.AdaptiveScale {
				session.AdaptiveScale = newScale
				if c.applyAdaptiveScaleLocked(session, true) {
					adapted = true
				}
			}
			nextInterval := clampDuration(session.FrameInterval+15*time.Millisecond, session.MinInterval, session.MaxInterval)
			if nextInterval != session.FrameInterval {
				session.FrameInterval = nextInterval
				adapted = true
			}
			nextTile := clampInt(session.TileSize+8, session.MinTile, session.MaxTile)
			if nextTile != session.TileSize {
				session.TileSize = nextTile
				adapted = true
			}
		}
	} else if improve {
		newScale := clampFloat(session.AdaptiveScale*1.1, session.MinScale, session.MaxScale)
		if newScale != session.AdaptiveScale {
			session.AdaptiveScale = newScale
			if c.applyAdaptiveScaleLocked(session, true) {
				adapted = true
			}
		}
		nextInterval := clampDuration(session.FrameInterval-10*time.Millisecond, session.MinInterval, session.MaxInterval)
		if nextInterval != session.FrameInterval {
			session.FrameInterval = nextInterval
			adapted = true
		}
		nextTile := clampInt(session.TileSize-4, session.MinTile, session.MaxTile)
		if nextTile != session.TileSize {
			session.TileSize = nextTile
			adapted = true
		}
	}

	if clipAdjusted {
		adapted = true
	}

	if adapted {
		session.LastAdaptation = now
	}
}

func (c *remoteDesktopSessionController) stream(ctx context.Context, session *RemoteDesktopSession) {
	var lastSent time.Time
	timer := time.NewTimer(0)
	defer timer.Stop()

	activeMode := normalizeStreamMode(session.Settings.Mode)
	var clipFrames []RemoteDesktopClipFrame
	var clipStart time.Time
	clipKeyPending := activeMode == RemoteStreamModeVideo
	clipBytes := 0
	var tileHasher remoteTileHasher

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		clipQuality := defaultClipQuality
		c.mu.Lock()
		if c.session == nil || c.session.ID != session.ID {
			prev := session.LastFrame
			c.mu.Unlock()
			releaseFrameBuffer(prev)
			return
		}

		c.refreshMonitorsLocked(session, false)

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
		targetInterval := session.FrameInterval
		mode := normalizeStreamMode(session.Settings.Mode)
		if mode != activeMode {
			activeMode = mode
			clipFrames = nil
			clipStart = time.Time{}
			clipBytes = 0
			clipKeyPending = mode == RemoteStreamModeVideo
			if session.LastFrame != nil {
				releaseFrameBuffer(session.LastFrame)
				session.LastFrame = nil
			}
			tileHasher.reset()
			session.ForceKeyFrame = true
		}

		minAllowed := session.MinClipQuality
		maxAllowed := session.MaxClipQuality
		if minAllowed <= 0 || minAllowed >= maxAllowed {
			minAllowed = minClipQuality
			maxAllowed = maxClipQuality
		}
		clipQuality = clampInt(session.ClipQuality, minAllowed, maxAllowed)
		if clipQuality <= 0 {
			clipQuality = clampInt(defaultClipQuality, minAllowed, maxAllowed)
		}

		forceKey := session.ForceKeyFrame || len(session.LastFrame) == 0
		prev := session.LastFrame
		var sequence uint64
		if mode == RemoteStreamModeImages {
			sequence = session.Sequence + 1
			session.Sequence = sequence
			session.ForceKeyFrame = false
		}
		if forceKey {
			tileHasher.reset()
		}
		c.mu.Unlock()

		if targetInterval <= 0 {
			targetInterval = 100 * time.Millisecond
		}

		processStart := time.Now()
		current, captureErr := captureMonitorFrame(monitor, width, height)
		if captureErr != nil {
			c.agent.logger.Printf("remote desktop capture error: %v", captureErr)
			c.mu.Lock()
			c.refreshMonitorsLocked(session, true)
			c.mu.Unlock()
			scheduleNextFrame(timer, targetInterval)
			continue
		}

		if mode == RemoteStreamModeVideo {
			clipDuration := clampDuration(targetInterval*2, minClipDuration, maxClipDuration)
			if clipDuration <= 0 {
				clipDuration = defaultClipDuration
			}

			if forceKey {
				clipFrames = nil
				clipStart = time.Now()
				clipBytes = 0
				clipKeyPending = true
			} else if clipStart.IsZero() {
				clipStart = time.Now()
			}

			if len(monitorsPayload) > 0 {
				clipKeyPending = true
			}

			offsetMs := 0
			if !clipStart.IsZero() {
				offsetMs = int(time.Since(clipStart).Milliseconds())
				if offsetMs < 0 {
					offsetMs = 0
				}
			}

			encoded, err := encodeJPEG(width, height, clipQuality, current)
			if err != nil {
				c.agent.logger.Printf("remote desktop clip encode error: %v", err)
				releaseFrameBuffer(current)
				releaseFrameBuffer(prev)
				scheduleNextFrame(timer, targetInterval)
				continue
			}

			clipBytes += len(encoded)
			clipFrames = append(clipFrames, RemoteDesktopClipFrame{
				OffsetMs: offsetMs,
				Width:    width,
				Height:   height,
				Encoding: remoteClipEncodingJPEG,
				Data:     encoded,
			})

			releaseFrameBuffer(current)
			releaseFrameBuffer(prev)

			clipElapsed := time.Since(clipStart)
			intervalMs := targetInterval.Milliseconds()
			if intervalMs <= 0 {
				intervalMs = 80
			}
			clipMs := clipDuration.Milliseconds()
			if clipMs <= 0 {
				clipMs = defaultClipDuration.Milliseconds()
			}
			frameCap := int(clipMs/int64(intervalMs)) + 1
			if frameCap < 2 {
				frameCap = 2
			}
			if frameCap > maxClipFrames {
				frameCap = maxClipFrames
			}

			shouldFlush := false
			if clipElapsed >= clipDuration || len(clipFrames) >= frameCap {
				shouldFlush = true
			}
			if clipKeyPending && len(clipFrames) > 0 {
				shouldFlush = true
			}
			if len(monitorsPayload) > 0 && len(clipFrames) > 0 {
				shouldFlush = true
			}

			if !shouldFlush {
				scheduleNextFrame(timer, targetInterval)
				continue
			}

			framesCopy := append([]RemoteDesktopClipFrame(nil), clipFrames...)
			durationMs := framesCopy[len(framesCopy)-1].OffsetMs
			if durationMs <= 0 {
				durationMs = int(clipElapsed.Milliseconds())
			}
			if durationMs <= 0 {
				durationMs = int(targetInterval.Milliseconds())
			}

			processingDuration := time.Since(processStart)
			frameDuration := targetInterval
			if !lastSent.IsZero() {
				if elapsed := time.Since(lastSent); elapsed > 0 {
					frameDuration = elapsed
				}
			}

			metrics := computeMetrics(targetInterval, frameDuration, processingDuration, clipBytes, clipQuality)
			timestamp := time.Now()
			frame := RemoteDesktopFramePacket{
				SessionID: session.ID,
				Timestamp: timestamp.UTC().Format(time.RFC3339Nano),
				Width:     width,
				Height:    height,
				KeyFrame:  clipKeyPending,
				Encoding:  remoteEncodingClip,
				Clip: &RemoteDesktopVideoClip{
					DurationMs: durationMs,
					Frames:     framesCopy,
				},
				Metrics: metrics,
			}
			if len(monitorsPayload) > 0 {
				frame.Monitors = monitorsPayload
			}

			nextInterval := targetInterval

			c.mu.Lock()
			if c.session != nil && c.session.ID == session.ID {
				c.session.Sequence++
				frame.Sequence = c.session.Sequence
				c.maybeAdaptQualityLocked(c.session, metrics, processingDuration, frameDuration, clipBytes)
				if len(monitorsPayload) > 0 {
					c.session.monitorsDirty = false
				}
				c.session.ForceKeyFrame = false
				nextInterval = c.session.FrameInterval
			} else {
				c.mu.Unlock()
				clipFrames = nil
				clipStart = time.Time{}
				clipBytes = 0
				clipKeyPending = true
				scheduleNextFrame(timer, targetInterval)
				continue
			}
			c.mu.Unlock()

			sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err = c.sendFrame(sendCtx, frame)
			cancel()
			if err != nil {
				if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
					c.agent.logger.Printf("remote desktop clip send error: %v", err)
				}
				clipFrames = nil
				clipStart = time.Time{}
				clipBytes = 0
				clipKeyPending = true
				scheduleNextFrame(timer, nextInterval)
				continue
			}

			clipFrames = nil
			clipStart = time.Time{}
			clipBytes = 0
			clipKeyPending = false
			scheduleNextFrame(timer, nextInterval)
			lastSent = timestamp
			continue
		}

		keyFrame := forceKey || len(prev) != len(current) || len(prev) == 0
		var imageData string
		var deltas []RemoteDesktopDeltaRect
		frameEncoding := remoteEncodingPNG
		bytesSent := 0

		if keyFrame {
			encoded, encoding, err := encodeKeyFrame(width, height, clipQuality, current)
			if err != nil {
				c.agent.logger.Printf("remote desktop encode frame: %v", err)
				releaseFrameBuffer(current)
				scheduleNextFrame(timer, targetInterval)
				continue
			}
			imageData = encoded
			frameEncoding = encoding
			bytesSent += len(encoded)
		} else {
			rects, fallback, err := diffFrames(prev, current, width, height, tile, &tileHasher, clipQuality)
			if err != nil {
				c.agent.logger.Printf("remote desktop diff error: %v", err)
				keyFrame = true
			} else if fallback {
				keyFrame = true
			} else {
				deltas = rects
				for _, rect := range rects {
					bytesSent += len(rect.Data)
				}
				if len(rects) == 0 {
					if time.Since(lastSent) > 3*targetInterval {
						keyFrame = true
					}
				}
			}

			if keyFrame && imageData == "" {
				if encoded, encoding, encErr := encodeKeyFrame(width, height, clipQuality, current); encErr == nil {
					imageData = encoded
					frameEncoding = encoding
					bytesSent += len(encoded)
				} else {
					c.agent.logger.Printf("remote desktop fallback encode: %v", encErr)
					releaseFrameBuffer(current)
					scheduleNextFrame(timer, targetInterval)
					continue
				}
			}
		}

		processingDuration := time.Since(processStart)
		frameDuration := targetInterval
		if !lastSent.IsZero() {
			if elapsed := time.Since(lastSent); elapsed > 0 {
				frameDuration = elapsed
			}
		}

		usedQuality := 0
		if frameEncoding == remoteEncodingJPEG {
			usedQuality = clipQuality
		} else if len(deltas) > 0 {
			for _, rect := range deltas {
				if rect.Encoding == remoteEncodingJPEG {
					usedQuality = clipQuality
					break
				}
			}
		}
		metrics := computeMetrics(targetInterval, frameDuration, processingDuration, bytesSent, usedQuality)
		timestamp := time.Now()
		frame := RemoteDesktopFramePacket{
			SessionID: session.ID,
			Sequence:  sequence,
			Timestamp: timestamp.UTC().Format(time.RFC3339Nano),
			Width:     width,
			Height:    height,
			KeyFrame:  keyFrame,
			Encoding:  frameEncoding,
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

		nextInterval := targetInterval

		c.mu.Lock()
		if c.session != nil && c.session.ID == session.ID {
			c.maybeAdaptQualityLocked(c.session, metrics, processingDuration, frameDuration, bytesSent)
			nextInterval = c.session.FrameInterval
		}
		c.mu.Unlock()

		sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := c.sendFrame(sendCtx, frame)
		cancel()
		if err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				c.agent.logger.Printf("remote desktop frame send error: %v", err)
			}
			releaseFrameBuffer(current)
			scheduleNextFrame(timer, nextInterval)
			continue
		}

		c.mu.Lock()
		updated := false
		updatedTile := tile
		updatedWidth := width
		updatedHeight := height
		if c.session != nil && c.session.ID == session.ID {
			c.session.LastFrame = current
			releaseFrameBuffer(prev)
			if len(monitorsPayload) > 0 {
				c.session.monitorsDirty = false
			}
			nextInterval = c.session.FrameInterval
			updatedTile = c.session.TileSize
			updatedWidth = c.session.Width
			updatedHeight = c.session.Height
			updated = true
		} else {
			releaseFrameBuffer(current)
			releaseFrameBuffer(prev)
		}
		c.mu.Unlock()

		if updated {
			if updatedWidth != width || updatedHeight != height {
				tileHasher.reset()
			} else if keyFrame || updatedTile != tile {
				tileHasher.rebuild(current, width, height, updatedTile)
			}
		}

		scheduleNextFrame(timer, nextInterval)
		lastSent = timestamp
	}
}

func (c *remoteDesktopSessionController) sendFrame(ctx context.Context, frame RemoteDesktopFramePacket) error {
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/api/agents/%s/remote-desktop/frames", c.agent.baseURL, url.PathEscape(c.agent.id))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent())
	if strings.TrimSpace(c.agent.key) != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.agent.key))
	}

	resp, err := c.agent.client.Do(req)
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
		Mode:     RemoteStreamModeVideo,
	}
}

func applySettingsPatch(settings *RemoteDesktopSettings, patch *RemoteDesktopSettingsPatch) {
	if patch == nil {
		return
	}
	if patch.Quality != nil {
		settings.Quality = normalizeQuality(*patch.Quality)
	}
	if patch.Mode != nil {
		settings.Mode = normalizeStreamMode(*patch.Mode)
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

func normalizeStreamMode(value RemoteDesktopStreamMode) RemoteDesktopStreamMode {
	switch strings.ToLower(string(value)) {
	case string(RemoteStreamModeImages):
		return RemoteStreamModeImages
	case string(RemoteStreamModeVideo):
		return RemoteStreamModeVideo
	default:
		return RemoteStreamModeVideo
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

func clipQualityBaseline(preset RemoteDesktopQuality) int {
	switch preset {
	case RemoteQualityHigh:
		return 88
	case RemoteQualityMedium:
		return 80
	case RemoteQualityLow:
		return 72
	default:
		return defaultClipQuality
	}
}

func captureMonitorFrame(monitor remoteMonitor, width, height int) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, errors.New("invalid frame dimensions")
	}

	img, err := safeCaptureRect(monitor.bounds)
	if err != nil {
		return nil, err
	}
	srcBounds := img.Bounds()
	if srcBounds.Dx() == 0 || srcBounds.Dy() == 0 {
		return nil, errors.New("empty monitor capture")
	}

	frameSize := width * height * 4
	buffer := acquireFrameBuffer(frameSize)

	if srcBounds.Dx() != width || srcBounds.Dy() != height {
		dest := image.RGBA{
			Pix:    buffer,
			Stride: width * 4,
			Rect:   image.Rect(0, 0, width, height),
		}
		xdraw.ApproxBiLinear.Scale(&dest, dest.Rect, img, srcBounds, xdraw.Src, nil)
		return buffer, nil
	}

	copyRGBAInto(buffer, img, width, height)
	return buffer, nil
}

func copyRGBAInto(dst []byte, img *image.RGBA, width, height int) {
	if len(dst) == 0 || img == nil {
		return
	}

	stride := img.Stride
	if stride == width*4 {
		copy(dst, img.Pix[:width*height*4])
		return
	}
	for y := 0; y < height; y++ {
		start := y * width * 4
		rowStart := y * stride
		copy(dst[start:start+width*4], img.Pix[rowStart:rowStart+width*4])
	}
}

func acquireFrameBuffer(size int) []byte {
	if size <= 0 {
		return nil
	}

	value := frameBufferPool.Get()
	if buf, ok := value.([]byte); ok {
		if cap(buf) >= size {
			return buf[:size]
		}
		frameBufferPool.Put(buf[:0])
		return make([]byte, size)
	}

	return make([]byte, size)
}

func releaseFrameBuffer(buf []byte) {
	if buf == nil {
		return
	}
	frameBufferPool.Put(buf[:0])
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
	if len(data) == 0 || width <= 0 || height <= 0 {
		return "", errors.New("invalid frame data")
	}

	img := &image.RGBA{
		Pix:    data,
		Stride: width * 4,
		Rect:   image.Rect(0, 0, width, height),
	}
	bufPtr := imageBufferPool.Get().(*bytes.Buffer)
	bufPtr.Reset()
	defer imageBufferPool.Put(bufPtr)

	if err := pngEncoder.Encode(bufPtr, img); err != nil {
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(bufPtr.Bytes())
	return encoded, nil
}

func encodeJPEG(width, height, quality int, data []byte) (string, error) {
	if len(data) == 0 || width <= 0 || height <= 0 {
		return "", errors.New("invalid frame data")
	}

	img := &image.RGBA{
		Pix:    data,
		Stride: width * 4,
		Rect:   image.Rect(0, 0, width, height),
	}
	bufPtr := imageBufferPool.Get().(*bytes.Buffer)
	bufPtr.Reset()
	defer imageBufferPool.Put(bufPtr)

	if quality <= 0 {
		quality = defaultClipQuality
	}
	optsPtr := jpegOptionsPool.Get().(*jpeg.Options)
	optsPtr.Quality = clampInt(quality, 1, 100)
	err := jpeg.Encode(bufPtr, img, optsPtr)
	jpegOptionsPool.Put(optsPtr)
	if err != nil {
		return "", err
	}

	encoded := base64.StdEncoding.EncodeToString(bufPtr.Bytes())
	return encoded, nil
}

func encodeKeyFrame(width, height, quality int, data []byte) (string, string, error) {
	useJPEG := shouldUseJPEGForKeyFrame(width, height, quality)
	if useJPEG {
		if encoded, err := encodeJPEG(width, height, quality, data); err == nil {
			return encoded, remoteEncodingJPEG, nil
		}
	}

	encoded, err := encodePNG(width, height, data)
	if err != nil {
		return "", remoteEncodingPNG, err
	}
	return encoded, remoteEncodingPNG, nil
}

func shouldUseJPEGForKeyFrame(width, height, quality int) bool {
	if width <= 0 || height <= 0 {
		return false
	}
	area := width * height
	if area >= jpegKeyFrameMinPixels {
		return true
	}
	if quality <= 0 {
		quality = defaultClipQuality
	}
	if quality >= 85 && area >= 240*180 {
		return true
	}
	return false
}

func encodeRegionPNG(data []byte, stride, x, y, w, h int) (string, error) {
	if stride <= 0 || w <= 0 || h <= 0 {
		return "", errors.New("invalid region dimensions")
	}

	start := y*stride + x*4
	if start < 0 || start >= len(data) {
		return "", errors.New("region start out of range")
	}

	needed := (h-1)*stride + w*4
	if start+needed > len(data) {
		return "", errors.New("region exceeds frame bounds")
	}

	region := image.RGBA{
		Pix:    data[start : start+needed],
		Stride: stride,
		Rect:   image.Rect(0, 0, w, h),
	}

	bufPtr := imageBufferPool.Get().(*bytes.Buffer)
	bufPtr.Reset()
	defer imageBufferPool.Put(bufPtr)

	if err := pngEncoder.Encode(bufPtr, &region); err != nil {
		return "", err
	}

	encoded := base64.StdEncoding.EncodeToString(bufPtr.Bytes())
	return encoded, nil
}

func encodeRegionJPEG(data []byte, stride, x, y, w, h, quality int) (string, error) {
	if stride <= 0 || w <= 0 || h <= 0 {
		return "", errors.New("invalid region dimensions")
	}

	start := y*stride + x*4
	if start < 0 || start >= len(data) {
		return "", errors.New("region start out of range")
	}

	needed := (h-1)*stride + w*4
	if start+needed > len(data) {
		return "", errors.New("region exceeds frame bounds")
	}

	region := image.RGBA{
		Pix:    data[start : start+needed],
		Stride: stride,
		Rect:   image.Rect(0, 0, w, h),
	}

	bufPtr := imageBufferPool.Get().(*bytes.Buffer)
	bufPtr.Reset()
	defer imageBufferPool.Put(bufPtr)

	if quality <= 0 {
		quality = defaultClipQuality
	}
	optsPtr := jpegOptionsPool.Get().(*jpeg.Options)
	optsPtr.Quality = clampInt(quality, 1, 100)
	err := jpeg.Encode(bufPtr, &region, optsPtr)
	jpegOptionsPool.Put(optsPtr)
	if err != nil {
		return "", err
	}

	encoded := base64.StdEncoding.EncodeToString(bufPtr.Bytes())
	return encoded, nil
}

func shouldUseJPEGForRegion(width, height, quality int) bool {
	if width <= 0 || height <= 0 {
		return false
	}
	area := width * height
	if area >= jpegDeltaMinPixels {
		return true
	}
	if quality <= 0 {
		quality = defaultClipQuality
	}
	if quality >= 85 && area >= 24*24 {
		return true
	}
	return false
}

func encodeTileRegion(data []byte, stride int, region tileRegion, quality int) (RemoteDesktopDeltaRect, error) {
	preferJPEG := shouldUseJPEGForRegion(region.w, region.h, quality)
	if preferJPEG {
		if encoded, err := encodeRegionJPEG(data, stride, region.x, region.y, region.w, region.h, quality); err == nil {
			return RemoteDesktopDeltaRect{
				X:        region.x,
				Y:        region.y,
				Width:    region.w,
				Height:   region.h,
				Encoding: remoteEncodingJPEG,
				Data:     encoded,
			}, nil
		}
	}

	encoded, err := encodeRegionPNG(data, stride, region.x, region.y, region.w, region.h)
	if err != nil {
		return RemoteDesktopDeltaRect{}, err
	}

	return RemoteDesktopDeltaRect{
		X:        region.x,
		Y:        region.y,
		Width:    region.w,
		Height:   region.h,
		Encoding: remoteEncodingPNG,
		Data:     encoded,
	}, nil
}

type tileRegion struct {
	x int
	y int
	w int
	h int
}

func diffFrames(previous, current []byte, width, height, tile int, state *remoteTileHasher, quality int) ([]RemoteDesktopDeltaRect, bool, error) {
	if state == nil {
		return diffFramesLegacy(previous, current, width, height, tile, quality)
	}

	if len(previous) != len(current) {
		return nil, false, errors.New("frame size mismatch")
	}
	if len(current) == 0 {
		return nil, false, nil
	}

	if bytes.Equal(previous, current) {
		state.ready = true
		return nil, false, nil
	}

	state.ensure(width, height, tile)
	stride := width * 4
	baselineReady := state.ready

	estimatedCols := (width + tile - 1) / tile
	estimatedRows := (height + tile - 1) / tile
	totalTiles := maxInt(1, estimatedCols*estimatedRows)
	regions := make([]tileRegion, 0, maxInt(1, totalTiles/4))

	maxRegions := maxInt(64, totalTiles/maxDeltaTileFactor)
	maxPixels := int(float64(width*height) * maxDeltaCoverageRatio)
	if maxPixels <= 0 {
		maxPixels = width * height
	}

	changedPixels := 0
	idx := 0
	for y := 0; y < height; y += tile {
		h := minInt(tile, height-y)
		for x := 0; x < width; x += tile {
			w := minInt(tile, width-x)
			sum := tileChecksum(current, stride, x, y, w, h)
			prevSum := uint64(0)
			if idx < len(state.checksums) {
				prevSum = state.checksums[idx]
			}
			state.checksums[idx] = sum
			idx++

			if baselineReady && prevSum == sum {
				continue
			}

			regions = append(regions, tileRegion{x: x, y: y, w: w, h: h})
			changedPixels += w * h
			if len(regions) > maxRegions || changedPixels > maxPixels {
				state.ready = false
				return nil, true, nil
			}
		}
	}

	if len(regions) == 0 {
		state.ready = true
		return nil, false, nil
	}

	regions = mergeTileRegions(regions)

	deltas, err := encodeRegions(current, stride, regions, quality)
	if err != nil {
		state.ready = false
		return nil, false, err
	}

	state.ready = true
	return deltas, false, nil
}

func diffFramesLegacy(previous, current []byte, width, height, tile int, quality int) ([]RemoteDesktopDeltaRect, bool, error) {
	if len(previous) != len(current) {
		return nil, false, errors.New("frame size mismatch")
	}
	if len(current) == 0 {
		return nil, false, nil
	}

	if bytes.Equal(previous, current) {
		return nil, false, nil
	}

	if tile <= 0 {
		tile = 32
	}

	stride := width * 4
	estimatedCols := (width + tile - 1) / tile
	estimatedRows := (height + tile - 1) / tile
	totalTiles := maxInt(1, estimatedCols*estimatedRows)
	regions := make([]tileRegion, 0, maxInt(1, totalTiles/4))

	maxRegions := maxInt(64, totalTiles/maxDeltaTileFactor)
	maxPixels := int(float64(width*height) * maxDeltaCoverageRatio)
	if maxPixels <= 0 {
		maxPixels = width * height
	}

	changedPixels := 0

	for y := 0; y < height; y += tile {
		h := minInt(tile, height-y)
		for x := 0; x < width; x += tile {
			w := minInt(tile, width-x)
			if regionChanged(previous, current, stride, x, y, w, h) {
				regions = append(regions, tileRegion{x: x, y: y, w: w, h: h})
				changedPixels += w * h
				if len(regions) > maxRegions || changedPixels > maxPixels {
					return nil, true, nil
				}
			}
		}
	}

	if len(regions) == 0 {
		return nil, false, nil
	}

	regions = mergeTileRegions(regions)

	deltas, err := encodeRegions(current, stride, regions, quality)
	if err != nil {
		return nil, false, err
	}
	return deltas, false, nil
}

func mergeTileRegions(regions []tileRegion) []tileRegion {
	if len(regions) <= 1 {
		return regions
	}

	merged := regions[:0]
	current := regions[0]
	for i := 1; i < len(regions); i++ {
		next := regions[i]
		if next.y == current.y && next.h == current.h && next.x == current.x+current.w {
			current.w += next.w
			continue
		}
		merged = append(merged, current)
		current = next
	}
	merged = append(merged, current)
	return merged
}

func encodeRegions(data []byte, stride int, regions []tileRegion, quality int) ([]RemoteDesktopDeltaRect, error) {
	deltas := make([]RemoteDesktopDeltaRect, len(regions))
	if len(regions) == 0 {
		return deltas, nil
	}

	workerCount := minInt(len(regions), maxInt(1, runtime.NumCPU()))
	jobs := make(chan int, len(regions))
	var wg sync.WaitGroup
	var encodeErr error
	var once sync.Once

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for idx := range jobs {
				region := regions[idx]
				rect, err := encodeTileRegion(data, stride, region, quality)
				if err != nil {
					once.Do(func() {
						encodeErr = err
					})
					return
				}
				deltas[idx] = rect
			}
		}()
	}

	for i := range regions {
		jobs <- i
	}
	close(jobs)

	wg.Wait()
	if encodeErr != nil {
		return nil, encodeErr
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

func tileChecksum(data []byte, stride, x, y, w, h int) uint64 {
	const (
		offsetBasis = 1469598103934665603
		prime       = 1099511628211
	)

	hash := uint64(offsetBasis)
	rowWidth := w * 4
	for row := 0; row < h; row++ {
		start := (y+row)*stride + x*4
		segment := data[start : start+rowWidth]
		for _, b := range segment {
			hash ^= uint64(b)
			hash *= prime
		}
	}
	return hash
}

func (c *remoteDesktopSessionController) refreshMonitorsLocked(session *RemoteDesktopSession, force bool) {
	if session == nil {
		return
	}

	if !force && !session.lastMonitorRefresh.IsZero() {
		if time.Since(session.lastMonitorRefresh) < monitorRefreshInterval {
			if len(session.monitorInfos) == 0 && len(session.monitors) > 0 {
				session.monitorInfos = monitorInfos(session.monitors)
			}
			return
		}
	}

	monitors := detectRemoteMonitors()
	if len(monitors) == 0 {
		rect := image.Rect(0, 0, 1280, 720)
		monitors = []remoteMonitor{{
			info:   RemoteDesktopMonitorInfo{ID: 0, Label: "Primary", Width: rect.Dx(), Height: rect.Dy()},
			bounds: rect,
		}}
	}

	now := time.Now()
	if !force && monitorsEquivalent(session.monitors, monitors) {
		session.lastMonitorRefresh = now
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
	session.lastMonitorRefresh = now
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

func computeMetrics(targetInterval, frameDuration, processing time.Duration, bytesSent int, clipQuality int) *RemoteDesktopFrameMetrics {
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
	if clipQuality > 0 {
		metrics.ClipQuality = clampInt(clipQuality, 1, 100)
	}

	return metrics
}

func scheduleNextFrame(timer *time.Timer, interval time.Duration) {
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(interval)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampInt(value, min, max int) int {
	if max < min {
		min, max = max, min
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func clampFloat(value, min, max float64) float64 {
	if max < min {
		min, max = max, min
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func clampDuration(value, min, max time.Duration) time.Duration {
	if max < min {
		min, max = max, min
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
