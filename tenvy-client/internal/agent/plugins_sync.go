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
	Manifest  *manifest.Manifest
	EntryPath string
	Staged    bool
}

type pluginStageHandler interface {
	Stage(context.Context, *Agent, manifest.ManifestDescriptor) (pluginStageOutcome, error)
}

type pluginStageRegistry struct {
	mu       sync.RWMutex
	handlers map[string]pluginStageHandler
	fallback pluginStageHandler
}

func newPluginStageRegistry() *pluginStageRegistry {
	registry := &pluginStageRegistry{handlers: make(map[string]pluginStageHandler)}
	registry.fallback = genericPluginStageHandler{}
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
	fallback := r.fallback
	r.mu.RUnlock()
	if handler != nil {
		return handler
	}
	return fallback
}

var pluginStages = newPluginStageRegistry()

type remoteDesktopStageHandler struct{}

type genericPluginStageHandler struct{}

func (genericPluginStageHandler) Stage(ctx context.Context, agent *Agent, descriptor manifest.ManifestDescriptor) (pluginStageOutcome, error) {
	var outcome pluginStageOutcome

	if agent == nil {
		return outcome, errors.New("agent not initialized")
	}
	if agent.plugins == nil || agent.client == nil {
		return outcome, errors.New("plugin staging unavailable")
	}

	result, err := plugins.StagePlugin(ctx, agent.plugins, agent.client, agent.baseURL, agent.id, agent.key, agent.userAgent(), agent.pluginRuntimeFacts(), descriptor)
	if err != nil {
		return outcome, err
	}
	outcome.Manifest = &result.Manifest
	outcome.EntryPath = result.EntryPath
	outcome.Staged = true
	return outcome, nil
}

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

	facts := agent.pluginRuntimeFacts()
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
	outcome.EntryPath = result.EntryPath
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

	endpoint := fmt.Sprintf("%s/api/clients/%s/plugins", strings.TrimRight(base, "/"), url.PathEscape(agentID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.tenvy.plugin-manifest+json")
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

		_, generic := handler.(genericPluginStageHandler)

		stageCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		var (
			manifestResult *manifest.Manifest
			entryPath      string
			stageErr       error
		)

		if generic {
			res, err := plugins.StagePlugin(stageCtx, a.plugins, a.client, a.baseURL, a.id, a.key, a.userAgent(), a.pluginRuntimeFacts(), entry)
			stageErr = err
			if err == nil {
				manifestResult = &res.Manifest
				entryPath = res.EntryPath
			}
		} else {
			outcome, err := handler.Stage(stageCtx, a, entry)
			stageErr = err
			if err == nil && outcome.Manifest != nil {
				manifestResult = outcome.Manifest
				entryPath = outcome.EntryPath
			}
		}
		if stageErr == nil && manifestResult != nil {
			if err := a.activatePlugin(stageCtx, *manifestResult, entryPath); err != nil {
				stageErr = err
				manifestResult = nil
			}
		}
		cancel()

		if stageErr != nil {
			resultErr = combineErrors(resultErr, fmt.Errorf("stage %s: %w", id, stageErr))
			if generic && a.plugins != nil {
				status := manifest.InstallError
				var stageError *plugins.StageError
				if errors.As(stageErr, &stageError) && stageError != nil {
					status = stageError.Status()
				}
				version := strings.TrimSpace(entry.Version)
				if stageError != nil && strings.TrimSpace(stageError.Version()) != "" {
					version = stageError.Version()
				}
				if err := plugins.RecordInstallStatus(a.plugins, id, version, status, stageErr.Error()); err != nil && a.logger != nil {
					a.logger.Printf("plugin sync: failed to record install status for %s: %v", id, err)
				}
			}
			continue
		}

		if manifestResult == nil {
			continue
		}

		if generic && a.plugins != nil {
			if err := plugins.ClearInstallStatus(a.plugins, manifestResult.ID); err != nil && a.logger != nil {
				a.logger.Printf("plugin sync: failed to clear install status for %s: %v", manifestResult.ID, err)
			}
		}

	}

	return resultErr
}

func (a *Agent) pluginRuntimeFacts() manifest.RuntimeFacts {
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

		if a.modules != nil {
			if err := a.modules.DeactivatePlugin(ctx, id); err != nil {
				resultErr = combineErrors(resultErr, fmt.Errorf("deactivate plugin %s: %w", id, err))
			}
			metadata := a.modules.Metadata()
			for _, moduleMeta := range metadata {
				removed := false
				for _, ext := range moduleMeta.Extensions {
					if strings.EqualFold(ext.Source, id) {
						if err := a.modules.UnregisterModuleExtension(moduleMeta.ID, id); err != nil {
							resultErr = combineErrors(resultErr, err)
						}
						removed = true
						break
					}
				}
				if removed {
					break
				}
			}
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

func (a *Agent) activatePlugin(ctx context.Context, mf manifest.Manifest, entryPath string) error {
	if a.modules == nil {
		return nil
	}
	pluginID := strings.TrimSpace(mf.ID)
	if pluginID == "" {
		return errors.New("manifest missing identifier")
	}
	entryPath = strings.TrimSpace(entryPath)
	if entryPath == "" {
		return fmt.Errorf("plugin %s entry path not resolved", pluginID)
	}
	info, err := os.Stat(entryPath)
	if err != nil {
		return fmt.Errorf("verify plugin entry: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("plugin entry %s is a directory", entryPath)
	}
	file, err := os.Open(entryPath)
	if err != nil {
		return fmt.Errorf("open plugin entry: %w", err)
	}
	file.Close()

	extensions := buildModuleExtensions(mf)
	return a.modules.ActivatePlugin(ctx, pluginID, extensions, PluginActivationFunc(func(context.Context) error { return nil }))
}

func buildModuleExtensions(mf manifest.Manifest) map[string]ModuleExtension {
	pluginID := strings.TrimSpace(mf.ID)
	version := strings.TrimSpace(mf.Version)
	if pluginID == "" {
		return nil
	}

	extensions := make(map[string]ModuleExtension)
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
		extension := extensions[moduleID]
		if extension.Source == "" {
			extension.Source = pluginID
			extension.Version = version
		}
		extension.Capabilities = append(extension.Capabilities, ModuleCapability{
			ID:          descriptor.ID,
			Name:        descriptor.Name,
			Description: descriptor.Description,
		})
		extensions[moduleID] = extension
	}
	return extensions
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
	if err := a.modules.DeactivatePlugin(ctx, plugins.RemoteDesktopEnginePluginID); err != nil {
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
