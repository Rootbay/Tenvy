package remotedesktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"math"
	"net"
	"net/url"
	"strings"
	"time"
)

var (
	errSessionStopped  = errors.New("remote desktop session stopped")
	errSessionReplaced = errors.New("remote desktop session replaced")
	errSessionShutdown = errors.New("remote desktop subsystem shutdown")
)

const (
	defaultFrameRequestTimeout = 10 * time.Second
	minFrameRequestTimeout     = 2 * time.Second
	maxFrameRequestTimeout     = 20 * time.Second
)

type frameEndpointCache struct {
	base     string
	agentID  string
	endpoint string
}

func NewRemoteDesktopStreamer(cfg Config) *RemoteDesktopStreamer {
	return &RemoteDesktopStreamer{
		controller: newRemoteDesktopSessionController(cfg),
	}
}

func newRemoteDesktopSessionController(cfg Config) *remoteDesktopSessionController {
	controller := &remoteDesktopSessionController{}
	controller.updateConfig(cfg)
	return controller
}

func (s *RemoteDesktopStreamer) UpdateConfig(cfg Config) {
	if s == nil || s.controller == nil {
		return
	}
	s.controller.updateConfig(cfg)
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

	for {
		c.mu.Lock()
		if c.session == nil || c.session.ID == sessionID {
			break
		}
		prev := c.stopLocked(errSessionReplaced)
		c.mu.Unlock()
		waitSession(prev)
	}

	if c.session != nil && c.session.ID == sessionID {
		c.applySettingsLocked(c.session, payload.Settings)
		c.mu.Unlock()
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
	streamCtx, cancel := context.WithCancelCause(context.Background())
	session := &RemoteDesktopSession{
		ID:            sessionID,
		Settings:      settings,
		ForceKeyFrame: true,
		monitors:      monitors,
		monitorInfos:  infos,
		monitorsDirty: true,
		ctx:           streamCtx,
		cancel:        cancel,
	}
	session.wg.Add(1)
	profile, ladder, idx := selectQualityProfile(settings.Quality, monitorInfo)
	session.qualityLadder = ladder
	session.ladderIndex = idx
	c.configureProfileLocked(session, monitorInfo, profile, true)
	c.session = session
	c.mu.Unlock()

	go c.stream(streamCtx, session)
	c.logf("remote desktop session %s started", sessionID)
	return nil
}

func (c *remoteDesktopSessionController) Stop(sessionID string) error {
	trimmed := strings.TrimSpace(sessionID)

	c.mu.Lock()
	if c.session == nil {
		c.mu.Unlock()
		return nil
	}
	if trimmed != "" && trimmed != c.session.ID {
		c.mu.Unlock()
		return fmt.Errorf("session %s not active", trimmed)
	}
	stopped := c.stopLocked(errSessionStopped)
	c.mu.Unlock()

	waitSession(stopped)
	if stopped != nil {
		c.logf("remote desktop session %s stopped", stopped.ID)
	}
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

func (c *remoteDesktopSessionController) Shutdown() {
	c.mu.Lock()
	stopped := c.stopLocked(errSessionShutdown)
	c.mu.Unlock()
	waitSession(stopped)
}

func (c *remoteDesktopSessionController) stopLocked(cause error) *RemoteDesktopSession {
	if c.session == nil {
		return nil
	}
	session := c.session
	if session.cancel != nil {
		if cause == nil {
			cause = errSessionStopped
		}
		session.cancel(cause)
	}
	c.session = nil
	return session
}

func waitSession(session *RemoteDesktopSession) {
	if session == nil {
		return
	}
	session.wg.Wait()
}

func (c *remoteDesktopSessionController) logf(format string, args ...interface{}) {
	cfg := c.config()
	if cfg.Logger == nil {
		return
	}
	cfg.Logger.Printf(format, args...)
}

func (c *remoteDesktopSessionController) userAgent() string {
	ua := strings.TrimSpace(c.config().UserAgent)
	if ua != "" {
		return ua
	}
	return "tenvy-client"
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func (c *remoteDesktopSessionController) applySettingsLocked(session *RemoteDesktopSession, patch *RemoteDesktopSettingsPatch) {
	if session == nil || patch == nil {
		return
	}

	prevMonitor := session.Settings.Monitor
	prevMode := session.Settings.Mode
	prevQuality := session.Settings.Quality
	qualityChanged := false

	if patch.Quality != nil {
		nextQuality := normalizeQuality(*patch.Quality)
		if nextQuality != session.Settings.Quality {
			qualityChanged = true
		}
		session.Settings.Quality = nextQuality
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

	if patch.Quality != nil && session.Settings.Quality != prevQuality {
		qualityChanged = true
	}
	profile, ladder, idx := selectQualityProfile(session.Settings.Quality, monitorInfo)
	session.qualityLadder = ladder
	session.ladderIndex = idx
	forceKey := session.Settings.Monitor != prevMonitor || qualityChanged
	c.configureProfileLocked(session, monitorInfo, profile, forceKey)
}

func (c *remoteDesktopSessionController) configureProfileLocked(
	session *RemoteDesktopSession,
	monitor RemoteDesktopMonitorInfo,
	profile remoteQualityProfile,
	forceKey bool,
) {
	if session == nil {
		return
	}

	if session.Settings.Mode == "" {
		session.Settings.Mode = RemoteStreamModeVideo
	}

	width := profile.width
	height := profile.height
	if width <= 0 {
		width = monitor.Width
	}
	if height <= 0 {
		height = monitor.Height
	}
	if width <= 0 {
		width = 1280
	}
	if height <= 0 {
		height = 720
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

	baseTile := profile.tile
	if baseTile <= 0 {
		baseTile = 40
	}
	session.BaseTile = baseTile
	session.MinTile = maxInt(24, baseTile-16)
	session.MaxTile = minInt(120, baseTile+32)
	session.TileSize = clampInt(baseTile, session.MinTile, session.MaxTile)

	session.MinClipQuality = minClipQuality
	session.MaxClipQuality = maxClipQuality
	baseClipQuality := profile.clipQuality
	if baseClipQuality <= 0 {
		baseClipQuality = clipQualityBaseline(session.Settings.Quality)
	}
	if baseClipQuality <= 0 {
		baseClipQuality = defaultClipQuality
	}
	baseClipQuality = clampInt(baseClipQuality, session.MinClipQuality, session.MaxClipQuality)
	session.BaseClipQuality = baseClipQuality
	if session.Settings.Mode != RemoteStreamModeVideo {
		session.ClipQuality = baseClipQuality
	} else {
		if session.ClipQuality == 0 || forceKey {
			session.ClipQuality = baseClipQuality
		} else {
			session.ClipQuality = clampInt(session.ClipQuality, session.MinClipQuality, session.MaxClipQuality)
		}
	}

	baseInterval := profile.interval
	if baseInterval <= 0 {
		baseInterval = 100 * time.Millisecond
	}
	session.BaseInterval = baseInterval
	session.MinInterval = maxDuration(50*time.Millisecond, baseInterval/2)
	session.MaxInterval = minDuration(400*time.Millisecond, baseInterval*2)
	session.FrameInterval = clampDuration(baseInterval, session.MinInterval, session.MaxInterval)

	session.TargetBitrateKbps = maxInt(0, profile.bitrate)

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
		session.bandwidthEMA = 0
		session.fpsEMA = 0
		session.processingEMA = 0
		session.frameDropEMA = 0
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
	if len(session.qualityLadder) == 0 {
		return
	}

	now := time.Now()
	if !session.LastAdaptation.IsZero() && now.Sub(session.LastAdaptation) < 1200*time.Millisecond {
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

	const emaAlpha = 0.35
	if fps > 0 {
		session.fpsEMA = updateEMA(session.fpsEMA, fps, emaAlpha)
	}
	if bandwidth > 0 {
		session.bandwidthEMA = updateEMA(session.bandwidthEMA, bandwidth, emaAlpha)
	}
	processingMs := processing.Seconds() * 1000
	if processingMs > 0 {
		session.processingEMA = updateEMA(session.processingEMA, processingMs, emaAlpha)
	}

	ladderIndex := clampInt(session.ladderIndex, 0, len(session.qualityLadder)-1)
	currentProfile := session.qualityLadder[ladderIndex]
	minLadderBitrate := 0
	maxLadderBitrate := 0
	if len(session.qualityLadder) > 0 {
		minLadderBitrate = session.qualityLadder[len(session.qualityLadder)-1].bitrate
		maxLadderBitrate = session.qualityLadder[0].bitrate
	}

	processingBudget := float64(session.FrameInterval.Milliseconds())
	if processingBudget <= 0 {
		processingBudget = float64(currentProfile.interval.Milliseconds())
	}
	if processingBudget <= 0 {
		processingBudget = 100
	}

	fpsSample := session.fpsEMA
	if fpsSample <= 0 {
		fpsSample = fps
	}
	bandwidthSample := session.bandwidthEMA
	if bandwidthSample <= 0 {
		bandwidthSample = bandwidth
	}
	processingSample := session.processingEMA
	if processingSample <= 0 {
		processingSample = processingMs
	}

	dropRate := clampFloat(session.frameDropEMA, 0, 1)

	degrade := false
	improve := false

	if fpsSample > 0 && fpsSample < 12 {
		degrade = true
	}
	if bandwidthSample > 0 && currentProfile.bitrate > 0 && bandwidthSample > float64(currentProfile.bitrate)*1.15 {
		degrade = true
	}
	if processingSample > 0 && processingBudget > 0 && processingSample > processingBudget*0.85 {
		degrade = true
	}
	if session.FrameInterval > 0 && frameDuration > session.FrameInterval+session.FrameInterval/2 {
		degrade = true
	}
	if dropRate > 0.12 {
		degrade = true
	}

	if !degrade && dropRate < 0.08 && session.ladderIndex > 0 {
		prevProfile := session.qualityLadder[session.ladderIndex-1]
		targetBandwidth := float64(prevProfile.bitrate)
		if targetBandwidth <= 0 {
			targetBandwidth = float64(currentProfile.bitrate)
		}
		if fpsSample >= 22 && processingSample < processingBudget*0.65 {
			if (targetBandwidth <= 0 || bandwidthSample <= 0 || bandwidthSample < targetBandwidth*0.78) && dropRate < 0.04 {
				improve = true
			}
		}
	}

	if dropRate > 0.08 {
		improve = false
	}

	if degrade {
		if session.ClipQuality > session.MinClipQuality {
			nextQuality := session.ClipQuality - clipQualityStepDown
			if nextQuality < session.MinClipQuality {
				nextQuality = session.MinClipQuality
			}
			nextQuality = clampInt(nextQuality, session.MinClipQuality, session.MaxClipQuality)
			if nextQuality < session.ClipQuality {
				session.ClipQuality = nextQuality
				session.LastAdaptation = now
				return
			}
		}
		if session.Settings.Mode == RemoteStreamModeImages && session.TileSize < session.MaxTile {
			nextTile := clampInt(session.TileSize+8, session.MinTile, session.MaxTile)
			if nextTile > session.TileSize {
				session.TileSize = nextTile
				session.LastAdaptation = now
				return
			}
		}
		if session.FrameInterval < session.MaxInterval {
			nextInterval := time.Duration(float64(session.FrameInterval) * 1.25)
			if nextInterval <= session.FrameInterval {
				nextInterval = session.FrameInterval + 15*time.Millisecond
			}
			nextInterval = clampDuration(nextInterval, session.MinInterval, session.MaxInterval)
			if nextInterval > session.FrameInterval {
				session.FrameInterval = nextInterval
				session.LastAdaptation = now
				return
			}
		}
		nextScale := clampFloat(session.AdaptiveScale*0.85, session.MinScale, session.MaxScale)
		if nextScale < session.AdaptiveScale-0.01 {
			session.AdaptiveScale = nextScale
			if c.applyAdaptiveScaleLocked(session, true) {
				session.LastAdaptation = now
				return
			}
		}
		if session.TargetBitrateKbps > 0 {
			lowerBound := minLadderBitrate
			if lowerBound <= 0 {
				lowerBound = int(float64(session.TargetBitrateKbps) * 0.5)
			}
			nextBitrate := int(math.Round(float64(session.TargetBitrateKbps) * 0.7))
			if lowerBound > 0 && nextBitrate < lowerBound {
				nextBitrate = lowerBound
			}
			if nextBitrate < session.TargetBitrateKbps {
				session.TargetBitrateKbps = nextBitrate
				session.LastAdaptation = now
				return
			}
		}
		if session.ladderIndex < len(session.qualityLadder)-1 {
			session.ladderIndex++
			session.AdaptiveScale = 1
			monitorIndex := clampMonitorIndex(session.monitors, session.Settings.Monitor)
			monitor := monitorInfoAt(session, monitorIndex)
			profile := session.qualityLadder[session.ladderIndex]
			c.configureProfileLocked(session, monitor, profile, true)
			session.LastAdaptation = now
			return
		}
	}

	if improve {
		if session.FrameInterval > session.MinInterval {
			target := session.BaseInterval
			if target <= 0 {
				target = session.MinInterval
			}
			nextInterval := time.Duration(float64(session.FrameInterval) * 0.85)
			if nextInterval < target {
				nextInterval = target
			}
			nextInterval = clampDuration(nextInterval, session.MinInterval, session.MaxInterval)
			if nextInterval < session.FrameInterval {
				session.FrameInterval = nextInterval
				session.LastAdaptation = now
				return
			}
		}
		if session.Settings.Mode == RemoteStreamModeImages && session.TileSize > session.MinTile {
			baseline := clampInt(session.BaseTile, session.MinTile, session.MaxTile)
			nextTile := clampInt(session.TileSize-6, session.MinTile, session.MaxTile)
			if nextTile < baseline {
				nextTile = baseline
			}
			if nextTile < session.TileSize {
				session.TileSize = nextTile
				session.LastAdaptation = now
				return
			}
		}
		if session.ClipQuality < session.MaxClipQuality {
			targetQuality := session.BaseClipQuality
			if targetQuality <= 0 {
				targetQuality = session.MaxClipQuality
			}
			nextQuality := session.ClipQuality + clipQualityStepUp
			if nextQuality > targetQuality {
				nextQuality = targetQuality
			}
			nextQuality = clampInt(nextQuality, session.MinClipQuality, session.MaxClipQuality)
			if nextQuality > session.ClipQuality {
				session.ClipQuality = nextQuality
				session.LastAdaptation = now
				return
			}
		}
		if session.TargetBitrateKbps > 0 {
			upperBound := maxLadderBitrate
			if upperBound <= 0 {
				upperBound = session.TargetBitrateKbps
			}
			step := maxInt(120, session.TargetBitrateKbps/10)
			nextBitrate := session.TargetBitrateKbps + step
			if upperBound > 0 && nextBitrate > upperBound {
				nextBitrate = upperBound
			}
			if nextBitrate > session.TargetBitrateKbps {
				session.TargetBitrateKbps = nextBitrate
				session.LastAdaptation = now
				return
			}
		}
		nextScale := clampFloat(session.AdaptiveScale+0.08, session.MinScale, session.MaxScale)
		if nextScale > session.AdaptiveScale+0.01 {
			session.AdaptiveScale = nextScale
			if c.applyAdaptiveScaleLocked(session, true) {
				session.LastAdaptation = now
				return
			}
		}
		if session.ladderIndex > 0 {
			session.ladderIndex--
			session.AdaptiveScale = 1
			monitorIndex := clampMonitorIndex(session.monitors, session.Settings.Monitor)
			monitor := monitorInfoAt(session, monitorIndex)
			profile := session.qualityLadder[session.ladderIndex]
			c.configureProfileLocked(session, monitor, profile, true)
			session.LastAdaptation = now
			return
		}
	}
}

func (c *remoteDesktopSessionController) updateConfig(cfg Config) {
	sanitized := sanitizeConfig(cfg)
	c.cfg.Store(sanitized)
	c.endpointCache.Store(frameEndpointCache{})
}

func (c *remoteDesktopSessionController) config() Config {
	if value := c.cfg.Load(); value != nil {
		return value.(Config)
	}
	return Config{}
}

func sanitizeConfig(cfg Config) Config {
	cfg.AgentID = strings.TrimSpace(cfg.AgentID)
	cfg.BaseURL = normalizeBaseURL(strings.TrimSpace(cfg.BaseURL))
	cfg.AuthKey = strings.TrimSpace(cfg.AuthKey)
	cfg.RequestTimeout = normalizeRequestTimeout(cfg.RequestTimeout)
	return cfg
}

func normalizeRequestTimeout(value time.Duration) time.Duration {
	if value <= 0 {
		return defaultFrameRequestTimeout
	}
	return clampDuration(value, minFrameRequestTimeout, maxFrameRequestTimeout)
}

func normalizeBaseURL(raw string) string {
	if raw == "" {
		return raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	if parsed.Scheme == "" {
		parsed.Scheme = "https"
	}
	parsed.Fragment = ""
	if parsed.User != nil {
		// Credentials should never be embedded in the base URL for security reasons.
		parsed.User = nil
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if parsed.Path == "/" {
		parsed.Path = ""
	}
	return parsed.String()
}

func (c *remoteDesktopSessionController) frameEndpoint(cfg Config) (string, error) {
	base := strings.TrimSpace(cfg.BaseURL)
	if base == "" {
		return "", errors.New("remote desktop: missing base URL")
	}

	agentID := strings.TrimSpace(cfg.AgentID)
	if agentID == "" {
		return "", errors.New("remote desktop: missing agent identifier")
	}

	if value := c.endpointCache.Load(); value != nil {
		if cached, ok := value.(frameEndpointCache); ok {
			if cached.base == base && cached.agentID == agentID && cached.endpoint != "" {
				return cached.endpoint, nil
			}
		}
	}

	parsed, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("remote desktop: invalid base URL: %w", err)
	}

	if err := enforceEndpointSecurity(parsed); err != nil {
		return "", err
	}

	pathRef := &url.URL{Path: fmt.Sprintf("/api/agents/%s/remote-desktop/frames", url.PathEscape(agentID))}
	endpoint := parsed.ResolveReference(pathRef).String()
	c.endpointCache.Store(frameEndpointCache{base: base, agentID: agentID, endpoint: endpoint})
	return endpoint, nil
}

func enforceEndpointSecurity(u *url.URL) error {
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "":
		u.Scheme = "https"
		scheme = "https"
	case "https":
	case "http":
		if !isLoopbackHost(u.Hostname()) {
			return fmt.Errorf("remote desktop: insecure http base URL %q", u.Redacted())
		}
	default:
		return fmt.Errorf("remote desktop: unsupported URL scheme %q", scheme)
	}

	if u.User != nil {
		return errors.New("remote desktop: base URL must not include credentials")
	}

	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return errors.New("remote desktop: invalid base URL host")
	}

	return nil
}

func isLoopbackHost(host string) bool {
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}
