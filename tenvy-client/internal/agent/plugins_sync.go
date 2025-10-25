package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rootbay/tenvy-client/internal/plugins"
	manifest "github.com/rootbay/tenvy-client/shared/pluginmanifest"
)

type pluginStageOutcome struct {
	Manifest *manifest.Manifest
	Staged   bool
}

type pluginStageHandler interface {
	Stage(context.Context, *Agent, manifest.ManifestDescriptor) (pluginStageOutcome, error)
}

type pluginStageRegistry struct {
	mu       sync.RWMutex
	handlers map[string]pluginStageHandler
}

func newPluginStageRegistry() *pluginStageRegistry {
	registry := &pluginStageRegistry{handlers: make(map[string]pluginStageHandler)}
	registry.Register(plugins.RemoteDesktopEnginePluginID, remoteDesktopStageHandler{})
	return registry
}

func (r *pluginStageRegistry) Register(id string, handler pluginStageHandler) {
	if r == nil {
		return
	}
	normalized := strings.ToLower(strings.TrimSpace(id))
	if normalized == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if handler == nil {
		delete(r.handlers, normalized)
		return
	}
	r.handlers[normalized] = handler
}

func (r *pluginStageRegistry) Unregister(id string) {
	r.Register(id, nil)
}

func (r *pluginStageRegistry) Lookup(id string) pluginStageHandler {
	if r == nil {
		return nil
	}
	normalized := strings.ToLower(strings.TrimSpace(id))
	if normalized == "" {
		return nil
	}
	r.mu.RLock()
	handler := r.handlers[normalized]
	r.mu.RUnlock()
	return handler
}

var pluginStages = newPluginStageRegistry()

type remoteDesktopStageHandler struct{}

func manifestDescriptorFingerprint(descriptor manifest.ManifestDescriptor) string {
	digest := strings.TrimSpace(descriptor.ManifestDigest)
	manual := strings.TrimSpace(descriptor.ManualPushAt)
	if manual == "" {
		return digest
	}
	if digest == "" {
		return manual
	}
	return fmt.Sprintf("%s:%s", digest, manual)
}

func (remoteDesktopStageHandler) Stage(ctx context.Context, agent *Agent, descriptor manifest.ManifestDescriptor) (pluginStageOutcome, error) {
	var outcome pluginStageOutcome

	if agent == nil {
		return outcome, errors.New("agent not initialized")
	}

	manualRequested := strings.TrimSpace(descriptor.ManualPushAt) != ""

	if !plugins.RemoteDesktopAutoSyncAllowed(descriptor) && !manualRequested {
		if agent.logger != nil {
			mode := strings.TrimSpace(string(descriptor.Distribution.DefaultMode))
			if mode == "" {
				mode = "unspecified"
			}
			agent.logger.Printf("plugin sync: skipping remote desktop plugin %s (delivery mode: %s, auto-update: %t)",
				strings.TrimSpace(descriptor.PluginID), mode, descriptor.Distribution.AutoUpdate)
		}
		return outcome, nil
	}

	facts := agent.remoteDesktopRuntimeFacts()
	result, err := plugins.StageRemoteDesktopEngine(
		ctx,
		agent.plugins,
		agent.client,
		agent.baseURL,
		agent.id,
		agent.key,
		agent.userAgent(),
		facts,
		descriptor,
	)
	if err != nil {
		return outcome, err
	}

	outcome.Manifest = &result.Manifest
	outcome.Staged = true
	return outcome, nil
}

func (a *Agent) fetchApprovedPluginList(ctx context.Context) (*manifest.ManifestList, error) {
	if a.client == nil {
		return nil, errors.New("http client not configured")
	}
	base := strings.TrimSpace(a.baseURL)
	agentID := strings.TrimSpace(a.id)
	if base == "" || agentID == "" {
		return nil, errors.New("agent identity not established")
	}

	endpoint := fmt.Sprintf("%s/api/agents/%s/plugins", strings.TrimRight(base, "/"), url.PathEscape(agentID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", a.userAgent())
	if key := strings.TrimSpace(a.key); key != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key))
	}
	applyRequestDecorations(req, a.requestHeaders, a.requestCookies)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("fetch plugin manifests: %s", message)
	}

	var snapshot manifest.ManifestList
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("decode manifest snapshot: %w", err)
	}
	return &snapshot, nil
}

func (a *Agent) refreshApprovedPlugins(ctx context.Context) error {
	if a.plugins == nil || a.client == nil {
		return nil
	}

	requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	snapshot, err := a.fetchApprovedPluginList(requestCtx)
	if err != nil {
		return err
	}
	a.setPluginManifestList(snapshot)
	return a.stagePluginsFromList(requestCtx, snapshot)
}

func (a *Agent) applyPluginManifestDelta(ctx context.Context, delta *manifest.ManifestDelta) error {
	if delta == nil {
		return nil
	}

	a.pluginManifestMu.RLock()
	currentVersion := a.pluginManifestVersion
	a.pluginManifestMu.RUnlock()

	if strings.TrimSpace(delta.Version) == currentVersion && len(delta.Updated) == 0 && len(delta.Removed) == 0 {
		return nil
	}

	removed := a.pluginIDsForRemoval(delta)
	var removalErr error
	if len(removed) > 0 {
		if err := a.handlePluginRemoval(ctx, removed); err != nil {
			removalErr = err
		}
	}

	if a.plugins == nil || a.client == nil {
		return removalErr
	}

	requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	snapshot, err := a.fetchApprovedPluginList(requestCtx)
	if err != nil {
		return combineErrors(removalErr, err)
	}
	a.setPluginManifestList(snapshot)
	stageErr := a.stagePluginsFromList(requestCtx, snapshot)
	return combineErrors(removalErr, stageErr)
}

func (a *Agent) stagePluginsFromList(ctx context.Context, snapshot *manifest.ManifestList) error {
	if snapshot == nil || len(snapshot.Manifests) == 0 {
		return nil
	}
	if a.plugins == nil || a.client == nil {
		return nil
	}

	var resultErr error
	for _, entry := range snapshot.Manifests {
		id := strings.TrimSpace(entry.PluginID)
		handler := pluginStages.Lookup(id)
		if handler == nil {
			continue
		}

		stageCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		outcome, err := handler.Stage(stageCtx, a, entry)
		cancel()
		if err != nil {
			resultErr = combineErrors(resultErr, fmt.Errorf("stage %s: %w", id, err))
			continue
		}

		if !outcome.Staged || outcome.Manifest == nil {
			continue
		}

		if err := a.registerPluginCapabilities(*outcome.Manifest); err != nil {
			resultErr = combineErrors(resultErr, fmt.Errorf("register capabilities for %s: %w", id, err))
		}
	}

	return resultErr
}

func (a *Agent) registerPluginCapabilities(mf manifest.Manifest) error {
	if a.modules == nil {
		return nil
	}
	mf.ID = strings.TrimSpace(mf.ID)
	mf.Version = strings.TrimSpace(mf.Version)
	if mf.ID == "" || len(mf.Capabilities) == 0 {
		return nil
	}

	extensions := make(map[string][]ModuleCapability)
	for _, capabilityID := range mf.Capabilities {
		capabilityID = strings.TrimSpace(capabilityID)
		if capabilityID == "" {
			continue
		}
		descriptor, ok := manifest.LookupCapability(capabilityID)
		if !ok {
			continue
		}
		moduleID := strings.TrimSpace(descriptor.Module)
		if moduleID == "" {
			continue
		}
		extensions[moduleID] = append(extensions[moduleID], ModuleCapability{
			ID:          descriptor.ID,
			Name:        descriptor.Name,
			Description: descriptor.Description,
		})
	}

	var resultErr error
	for moduleID, caps := range extensions {
		extension := ModuleExtension{
			Source:       mf.ID,
			Version:      mf.Version,
			Capabilities: caps,
		}
		if err := a.modules.RegisterModuleExtension(moduleID, extension); err != nil {
			resultErr = combineErrors(resultErr, err)
		}
	}
	return resultErr
}

func (a *Agent) remoteDesktopRuntimeFacts() manifest.RuntimeFacts {
	metadata := a.metadata
	version := strings.TrimSpace(metadata.Version)
	if version == "" {
		version = strings.TrimSpace(a.buildVersion)
	}

	var activeModules []string
	if a.modules != nil {
		meta := a.modules.Metadata()
		activeModules = make([]string, 0, len(meta))
		for _, entry := range meta {
			if id := strings.TrimSpace(entry.ID); id != "" {
				activeModules = append(activeModules, id)
			}
		}
	}

	return manifest.RuntimeFacts{
		Platform:       metadata.OS,
		Architecture:   metadata.Architecture,
		AgentVersion:   version,
		EnabledModules: append([]string(nil), activeModules...),
	}
}

func (a *Agent) pluginIDsForRemoval(delta *manifest.ManifestDelta) []string {
	if delta == nil || len(delta.Removed) == 0 {
		return nil
	}

	snapshot := a.pluginManifestSnapshot()
	lowercase := make(map[string]string, len(snapshot))
	for id := range snapshot {
		lowered := strings.ToLower(strings.TrimSpace(id))
		if lowered == "" {
			continue
		}
		lowercase[lowered] = id
	}

	seen := make(map[string]struct{}, len(delta.Removed))
	var ids []string
	for _, raw := range delta.Removed {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		candidate := trimmed
		if actual, ok := lowercase[strings.ToLower(trimmed)]; ok {
			candidate = actual
		}
		normalized := strings.ToLower(candidate)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		ids = append(ids, candidate)
	}
	return ids
}

func (a *Agent) handlePluginRemoval(ctx context.Context, pluginIDs []string) error {
	if len(pluginIDs) == 0 {
		return nil
	}

	a.removePluginManifestEntries(pluginIDs)

	var resultErr error
	pluginRoot := ""
	if a.plugins != nil {
		pluginRoot = strings.TrimSpace(a.plugins.Root())
	}
	remoteRemoved := false

	for _, rawID := range pluginIDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if strings.EqualFold(id, plugins.RemoteDesktopEnginePluginID) {
			remoteRemoved = true
		}

		if a.plugins != nil {
			if err := plugins.ClearInstallStatus(a.plugins, id); err != nil {
				resultErr = combineErrors(resultErr, fmt.Errorf("clear install status for %s: %w", id, err))
			}
			if pluginRoot != "" {
				dir := filepath.Join(pluginRoot, id)
				if err := os.RemoveAll(dir); err != nil {
					resultErr = combineErrors(resultErr, fmt.Errorf("remove plugin %s: %w", id, err))
				}
			}
		}
	}

	if remoteRemoved {
		if err := a.resetRemoteDesktopEngine(ctx); err != nil {
			resultErr = combineErrors(resultErr, err)
		}
	}

	return resultErr
}

func (a *Agent) removePluginManifestEntries(pluginIDs []string) {
	if len(pluginIDs) == 0 {
		return
	}

	a.pluginManifestMu.Lock()
	defer a.pluginManifestMu.Unlock()

	if len(a.pluginManifestDigests) == 0 && len(a.pluginManifestDescriptors) == 0 {
		return
	}

	lookup := make(map[string]string)
	for id := range a.pluginManifestDescriptors {
		lowered := strings.ToLower(strings.TrimSpace(id))
		if lowered != "" {
			lookup[lowered] = id
		}
	}
	for id := range a.pluginManifestDigests {
		lowered := strings.ToLower(strings.TrimSpace(id))
		if lowered != "" {
			if _, ok := lookup[lowered]; !ok {
				lookup[lowered] = id
			}
		}
	}

	for _, raw := range pluginIDs {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		key := trimmed
		if actual, ok := lookup[strings.ToLower(trimmed)]; ok {
			key = actual
		}
		if a.pluginManifestDescriptors != nil {
			delete(a.pluginManifestDescriptors, key)
		}
		if a.pluginManifestDigests != nil {
			delete(a.pluginManifestDigests, key)
		}
	}
}

func (a *Agent) resetRemoteDesktopEngine(ctx context.Context) error {
	if a.modules == nil {
		return nil
	}

	module := a.modules.remoteDesktopModule()
	if module == nil {
		return nil
	}

	module.mu.Lock()
	previous := module.engine
	module.engine = nil
	module.requiredVersion = ""
	module.mu.Unlock()

	if previous != nil {
		previous.Shutdown()
	}

	var resultErr error
	if err := a.modules.UnregisterModuleExtension("remote-desktop", plugins.RemoteDesktopEnginePluginID); err != nil {
		resultErr = combineErrors(resultErr, err)
	}

	runtime := a.moduleRuntime()
	if ctx == nil {
		ctx = context.Background()
	}
	if err := module.configure(ctx, runtime); err != nil {
		resultErr = combineErrors(resultErr, err)
	}
	return resultErr
}

func combineErrors(a, b error) error {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return errors.Join(a, b)
}
