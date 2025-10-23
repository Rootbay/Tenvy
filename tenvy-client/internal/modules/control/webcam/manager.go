package webcam

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

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

type Manager struct {
	cfg atomic.Value // stores Config
}

func NewManager(cfg Config) *Manager {
	manager := &Manager{}
	manager.updateConfig(cfg)
	return manager
}

func (m *Manager) UpdateConfig(cfg Config) {
	if m == nil {
		return
	}
	m.updateConfig(cfg)
}

func (m *Manager) updateConfig(cfg Config) {
	m.cfg.Store(cfg)
}

func (m *Manager) config() Config {
	if value := m.cfg.Load(); value != nil {
		return value.(Config)
	}
	return Config{}
}

func (m *Manager) userAgent() string {
	cfg := m.config()
	ua := strings.TrimSpace(cfg.UserAgent)
	if ua != "" {
		return ua
	}
	return "tenvy-client"
}

func (m *Manager) logf(format string, args ...interface{}) {
	if m == nil {
		return
	}
	cfg := m.config()
	if cfg.Logger == nil {
		return
	}
	cfg.Logger.Printf(format, args...)
}

func (m *Manager) HandleCommand(ctx context.Context, cmd Command) CommandResult {
	completedAt := time.Now().UTC().Format(time.RFC3339Nano)

	var payload protocol.WebcamCommandPayload
	if len(cmd.Payload) > 0 {
		if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
			return CommandResult{
				CommandID:   cmd.ID,
				Success:     false,
				Error:       fmt.Sprintf("invalid webcam control payload: %v", err),
				CompletedAt: completedAt,
			}
		}
	}

	action := strings.TrimSpace(strings.ToLower(payload.Action))
	var err error

	switch action {
	case "enumerate", "inventory":
		err = m.publishInventory(ctx, payload.RequestID)
	case "start", "stop", "update":
		err = errors.New("webcam streaming is not supported on this agent")
	case "":
		err = errors.New("missing webcam control action")
	default:
		err = fmt.Errorf("unsupported webcam control action: %s", payload.Action)
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

func (m *Manager) publishInventory(ctx context.Context, requestID string) error {
	devices, warning, err := captureWebcamInventory()
	if err != nil {
		return err
	}

	inventory := protocol.WebcamDeviceInventory{
		Devices:    devices,
		CapturedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if trimmed := strings.TrimSpace(requestID); trimmed != "" {
		inventory.RequestID = trimmed
	}
	if warning != "" {
		inventory.Warning = warning
	}

	data, err := json.Marshal(inventory)
	if err != nil {
		return err
	}

	cfg := m.config()
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return errors.New("webcam control: missing base URL")
	}
	if cfg.Client == nil {
		return errors.New("webcam control: missing http client")
	}

	endpoint := fmt.Sprintf("%s/api/agents/%s/webcam/devices", baseURL, url.PathEscape(cfg.AgentID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if ua := strings.TrimSpace(m.userAgent()); ua != "" {
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
		return fmt.Errorf("webcam inventory publish failed: status %d", resp.StatusCode)
	}

	return nil
}
