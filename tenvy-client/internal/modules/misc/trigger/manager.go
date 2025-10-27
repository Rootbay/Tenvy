package trigger

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

type (
	Command       = protocol.Command
	CommandResult = protocol.CommandResult
)

type clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// Manager maintains trigger monitor configuration and synthetic metrics.
type Manager struct {
	mu      sync.Mutex
	clock   clock
	started time.Time
	config  monitorConfig
}

type monitorConfig struct {
	Feed               string       `json:"feed"`
	RefreshSeconds     int          `json:"refreshSeconds"`
	IncludeScreenshots bool         `json:"includeScreenshots"`
	IncludeCommands    bool         `json:"includeCommands"`
	Watchlist          []watchEntry `json:"watchlist"`
	LastUpdatedAt      string       `json:"lastUpdatedAt"`
}

type commandPayload struct {
	Action string         `json:"action"`
	Config monitorCommand `json:"config,omitempty"`
}

type monitorCommand struct {
	Feed               string       `json:"feed"`
	RefreshSeconds     int          `json:"refreshSeconds"`
	IncludeScreenshots bool         `json:"includeScreenshots"`
	IncludeCommands    bool         `json:"includeCommands"`
	Watchlist          []watchEntry `json:"watchlist"`
}

type watchEntry struct {
	Kind         string `json:"kind"`
	ID           string `json:"id"`
	DisplayName  string `json:"displayName"`
	AlertOnOpen  bool   `json:"alertOnOpen"`
	AlertOnClose bool   `json:"alertOnClose"`
}

type metric struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value string `json:"value"`
}

type statusResult struct {
	Config      monitorConfig `json:"config"`
	Metrics     []metric      `json:"metrics"`
	GeneratedAt string        `json:"generatedAt"`
}

func NewManager() *Manager {
	now := time.Now().UTC()
	return &Manager{
		clock:   systemClock{},
		started: now,
		config: monitorConfig{
			Feed:               "live",
			RefreshSeconds:     5,
			IncludeScreenshots: false,
			IncludeCommands:    true,
			Watchlist:          []watchEntry{},
			LastUpdatedAt:      now.Format(time.RFC3339),
		},
	}
}

func (m *Manager) HandleCommand(ctx context.Context, cmd Command) CommandResult {
	completedAt := time.Now().UTC().Format(time.RFC3339Nano)
	result := CommandResult{CommandID: cmd.ID, CompletedAt: completedAt}

	var payload commandPayload
	if len(cmd.Payload) > 0 {
		if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("invalid trigger monitor payload: %v", err)
			return result
		}
	}

	action := strings.TrimSpace(strings.ToLower(payload.Action))
	if action == "" {
		action = "status"
	}

	switch action {
	case "status":
		if err := m.writeStatus(&result); err != nil {
			result.Success = false
			result.Error = err.Error()
			return result
		}
	case "configure":
		if err := m.applyConfig(payload.Config, &result); err != nil {
			result.Success = false
			result.Error = err.Error()
			return result
		}
	default:
		result.Success = false
		result.Error = fmt.Sprintf("unsupported trigger monitor action: %s", payload.Action)
		return result
	}

	result.Success = true
	return result
}

func (m *Manager) writeStatus(result *CommandResult) error {
	status := statusResult{
		Config:      m.currentConfig(),
		Metrics:     m.collectMetrics(),
		GeneratedAt: m.now().Format(time.RFC3339),
	}

	payload, err := json.Marshal(map[string]any{
		"action": "status",
		"status": "ok",
		"result": status,
	})
	if err != nil {
		return err
	}
	result.Output = string(payload)
	return nil
}

func (m *Manager) applyConfig(cfg monitorCommand, result *CommandResult) error {
	feed := strings.ToLower(strings.TrimSpace(cfg.Feed))
	if feed != "batch" {
		feed = "live"
	}
	refresh := cfg.RefreshSeconds
	if refresh <= 0 {
		refresh = 5
	}
	if feed == "live" && refresh < 2 {
		refresh = 2
	}
	if feed == "batch" && refresh < 30 {
		refresh = 30
	}

	updated := monitorConfig{
		Feed:               feed,
		RefreshSeconds:     refresh,
		IncludeScreenshots: cfg.IncludeScreenshots,
		IncludeCommands:    cfg.IncludeCommands,
		Watchlist:          sanitizeWatchlist(cfg.Watchlist),
		LastUpdatedAt:      m.now().Format(time.RFC3339),
	}

	m.mu.Lock()
	m.config = updated
	m.mu.Unlock()

	status := statusResult{
		Config:      updated,
		Metrics:     m.collectMetrics(),
		GeneratedAt: updated.LastUpdatedAt,
	}

	payload, err := json.Marshal(map[string]any{
		"action": "configure",
		"status": "ok",
		"result": status,
	})
	if err != nil {
		return err
	}
	result.Output = string(payload)
	return nil
}

func (m *Manager) currentConfig() monitorConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg := m.config
	cfg.Watchlist = cloneWatchlist(cfg.Watchlist)
	return cfg
}

func (m *Manager) collectMetrics() []metric {
	now := m.now()
	uptime := now.Sub(m.started)

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	metrics := []metric{
		{ID: "goroutines", Label: "Goroutines", Value: fmt.Sprintf("%d", runtime.NumGoroutine())},
		{ID: "heap", Label: "Heap Alloc", Value: fmt.Sprintf("%.2f MB", float64(mem.Alloc)/1024.0/1024.0)},
		{ID: "uptime", Label: "Agent Uptime", Value: uptime.Truncate(time.Second).String()},
	}
	return metrics
}

func sanitizeWatchlist(entries []watchEntry) []watchEntry {
	if len(entries) == 0 {
		return []watchEntry{}
	}

	sanitized := make([]watchEntry, 0, len(entries))
	for _, entry := range entries {
		kind := strings.ToLower(strings.TrimSpace(entry.Kind))
		if kind != "app" && kind != "url" {
			continue
		}

		id := strings.TrimSpace(entry.ID)
		if id == "" {
			continue
		}

		name := strings.TrimSpace(entry.DisplayName)
		if name == "" {
			continue
		}

		sanitized = append(sanitized, watchEntry{
			Kind:         kind,
			ID:           id,
			DisplayName:  name,
			AlertOnOpen:  entry.AlertOnOpen,
			AlertOnClose: entry.AlertOnClose,
		})
	}

	if len(sanitized) == 0 {
		return []watchEntry{}
	}

	return sanitized
}

func cloneWatchlist(entries []watchEntry) []watchEntry {
	if len(entries) == 0 {
		return []watchEntry{}
	}

	clone := make([]watchEntry, len(entries))
	copy(clone, entries)
	return clone
}

func (m *Manager) now() time.Time {
	if m.clock == nil {
		m.clock = systemClock{}
	}
	return m.clock.Now()
}

func (m *Manager) setClock(c clock) {
	if c == nil {
		m.clock = systemClock{}
		return
	}
	m.clock = c
}
