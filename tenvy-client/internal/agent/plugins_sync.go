package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rootbay/tenvy-client/internal/plugins"
	manifest "github.com/rootbay/tenvy-client/shared/pluginmanifest"
)

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

func (a *Agent) stagePluginsFromList(ctx context.Context, snapshot *manifest.ManifestList) error {
	if snapshot == nil || len(snapshot.Manifests) == 0 {
		return nil
	}
	if a.plugins == nil || a.client == nil {
		return nil
	}

	var resultErr error
	for _, entry := range snapshot.Manifests {
		if !strings.EqualFold(strings.TrimSpace(entry.PluginID), plugins.RemoteDesktopEnginePluginID) {
			continue
		}

		stageCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		facts := a.remoteDesktopRuntimeFacts()
		if _, err := plugins.StageRemoteDesktopEngine(
			stageCtx,
			a.plugins,
			a.client,
			a.baseURL,
			a.id,
			a.key,
			a.userAgent(),
			facts,
			entry,
		); err != nil {
			if resultErr == nil {
				resultErr = err
			}
		}
		cancel()
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
