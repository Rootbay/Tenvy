package plugins

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	manifest "github.com/rootbay/tenvy-client/shared/pluginmanifest"
)

// StageResult describes the outcome of a generic plugin staging operation.
type StageResult struct {
	Manifest manifest.Manifest
	Updated  bool
}

// StageError conveys the plugin installation status associated with a staging
// failure so callers can persist telemetry for the controller.
type StageError struct {
	status  manifest.PluginInstallStatus
	version string
	err     error
}

// Error implements the error interface.
func (e *StageError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

// Unwrap exposes the wrapped error value for errors.Is/As.
func (e *StageError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// Status returns the associated plugin installation status code.
func (e *StageError) Status() manifest.PluginInstallStatus {
	if e == nil {
		return manifest.InstallError
	}
	return e.status
}

// Version returns the plugin version associated with the failure, if known.
func (e *StageError) Version() string {
	if e == nil {
		return ""
	}
	return e.version
}

func newStageError(status manifest.PluginInstallStatus, version string, err error) error {
	if err == nil {
		return nil
	}
	return &StageError{
		status:  normalizeInstallStatus(status),
		version: strings.TrimSpace(version),
		err:     err,
	}
}

// StagePlugin stages a plugin described by the provided manifest descriptor,
// downloading the manifest and artifact from the controller, verifying
// signatures and hashes, and activating the staged installation under the
// manager root.
func StagePlugin(
	ctx context.Context,
	manager *Manager,
	client HTTPDoer,
	baseURL, agentID, authKey, userAgent string,
	runtimeFacts manifest.RuntimeFacts,
	descriptor manifest.ManifestDescriptor,
) (StageResult, error) {
	var result StageResult

	if manager == nil {
		return result, newStageError(manifest.InstallError, descriptor.Version, errors.New("plugin manager not initialized"))
	}
	if client == nil {
		return result, newStageError(manifest.InstallError, descriptor.Version, errors.New("http client not provided"))
	}

	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return result, newStageError(manifest.InstallError, descriptor.Version, errors.New("controller base url not provided"))
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return result, newStageError(manifest.InstallError, descriptor.Version, errors.New("agent identifier not provided"))
	}

	pluginID := strings.TrimSpace(descriptor.PluginID)
	if pluginID == "" {
		return result, newStageError(manifest.InstallError, descriptor.Version, errors.New("plugin id not provided"))
	}

	manualRequested := strings.TrimSpace(descriptor.ManualPushAt) != ""
	if !autoSyncAllowed(descriptor) && !manualRequested {
		message := "plugin automatic staging disabled by policy"
		return result, newStageError(manifest.InstallDisabled, descriptor.Version, errors.New(message))
	}

	manager.stageMu.Lock()
	defer manager.stageMu.Unlock()

	if err := os.MkdirAll(manager.root, 0o755); err != nil {
		return result, newStageError(manifest.InstallError, descriptor.Version, fmt.Errorf("ensure plugin root: %w", err))
	}

	manifestURL, artifactURL := pluginEndpoints(baseURL, agentID, pluginID)

	manifestData, mf, err := fetchPluginManifest(
		ctx,
		client,
		manifestURL,
		authKey,
		userAgent,
		descriptor.ManifestDigest,
	)
	if err != nil {
		return result, newStageError(manifest.InstallError, descriptor.Version, err)
	}
	result.Manifest = mf

	if trimmed := strings.TrimSpace(mf.ID); trimmed != "" {
		if !strings.EqualFold(trimmed, pluginID) {
			return result, newStageError(manifest.InstallError, mf.Version, fmt.Errorf("unexpected manifest id %s", mf.ID))
		}
		pluginID = trimmed
	}

	verificationResult, verifyErr := manifest.VerifySignature(mf, manager.verificationOptions())
	if verifyErr != nil {
		message := fmt.Sprintf("signature verification failed: %s", signatureErrorMessage(verifyErr))
		return result, newStageError(manifest.InstallBlocked, mf.Version, errors.New(message))
	}
	if verificationResult == nil || !verificationResult.Trusted {
		message := fmt.Sprintf("signature not trusted: %s", signatureUntrustedReason(mf, verificationResult))
		return result, newStageError(manifest.InstallBlocked, mf.Version, errors.New(message))
	}

	if err := manifest.CheckRuntimeCompatibility(mf, runtimeFacts); err != nil {
		message := fmt.Sprintf("plugin requirements not satisfied: %s", err.Error())
		return result, newStageError(manifest.InstallBlocked, mf.Version, errors.New(message))
	}

	artifactRel := filepath.Clean(filepath.FromSlash(mf.Package.Artifact))
	if artifactRel == "" || strings.HasPrefix(artifactRel, "..") {
		return result, newStageError(manifest.InstallError, mf.Version, errors.New("manifest artifact path is invalid"))
	}

	pluginDir := filepath.Join(manager.root, pluginID)
	manifestPath := filepath.Join(pluginDir, manifestFileName)
	artifactPath := filepath.Join(pluginDir, artifactRel)

	if upToDate, err := genericInstallationUpToDate(manifestPath, artifactPath, manifestData, mf); err == nil && upToDate {
		result.Updated = false
		return result, nil
	}

	stagingDir, err := os.MkdirTemp(manager.root, fmt.Sprintf("%s-", pluginID))
	if err != nil {
		return result, newStageError(manifest.InstallError, mf.Version, fmt.Errorf("create staging directory: %w", err))
	}
	cleanup := true
	defer func() {
		if cleanup {
			os.RemoveAll(stagingDir)
		}
	}()

	stagingManifest := filepath.Join(stagingDir, manifestFileName)
	if err := os.WriteFile(stagingManifest, manifestData, 0o644); err != nil {
		return result, newStageError(manifest.InstallError, mf.Version, fmt.Errorf("write manifest: %w", err))
	}

	stagingArtifact := filepath.Join(stagingDir, artifactRel)
	if err := os.MkdirAll(filepath.Dir(stagingArtifact), 0o755); err != nil {
		return result, newStageError(manifest.InstallError, mf.Version, fmt.Errorf("prepare artifact directory: %w", err))
	}

	if err := downloadPluginArtifact(ctx, client, artifactURL, authKey, userAgent, stagingArtifact); err != nil {
		return result, newStageError(manifest.InstallError, mf.Version, err)
	}

	if hash := strings.TrimSpace(mf.Package.Hash); hash != "" {
		sum, hashErr := fileHash(stagingArtifact)
		if hashErr != nil {
			return result, newStageError(manifest.InstallError, mf.Version, fmt.Errorf("compute artifact hash: %v", hashErr))
		}
		if !strings.EqualFold(hash, sum) {
			return result, newStageError(manifest.InstallError, mf.Version, errors.New("artifact hash mismatch"))
		}
	}

	if err := os.RemoveAll(pluginDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return result, newStageError(manifest.InstallError, mf.Version, fmt.Errorf("remove previous installation: %w", err))
	}

	if err := os.Rename(stagingDir, pluginDir); err != nil {
		return result, newStageError(manifest.InstallError, mf.Version, fmt.Errorf("activate staged plugin: %w", err))
	}
	cleanup = false

	result.Updated = true
	return result, nil
}

func autoSyncAllowed(descriptor manifest.ManifestDescriptor) bool {
	mode := strings.TrimSpace(string(descriptor.Distribution.DefaultMode))
	switch {
	case strings.EqualFold(mode, string(manifest.DeliveryAutomatic)):
		return true
	case strings.EqualFold(mode, string(manifest.DeliveryManual)):
		return false
	case mode == "":
		return descriptor.Distribution.AutoUpdate
	default:
		return false
	}
}

func pluginEndpoints(baseURL, agentID, pluginID string) (string, string) {
	trimmed := strings.TrimRight(baseURL, "/")
	encodedAgent := url.PathEscape(agentID)
	manifestURL := fmt.Sprintf("%s/api/agents/%s/plugins/%s", trimmed, encodedAgent, url.PathEscape(pluginID))
	artifactURL := fmt.Sprintf("%s/artifact", manifestURL)
	return manifestURL, artifactURL
}

func fetchPluginManifest(
	ctx context.Context,
	client HTTPDoer,
	endpoint, authKey, userAgent, expectedDigest string,
) ([]byte, manifest.Manifest, error) {
	var mf manifest.Manifest

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, mf, fmt.Errorf("create manifest request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if userAgent = strings.TrimSpace(userAgent); userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	if auth := strings.TrimSpace(authKey); auth != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", auth))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, mf, fmt.Errorf("fetch manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return nil, mf, fmt.Errorf("fetch manifest: %s", message)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, mf, fmt.Errorf("read manifest response: %w", err)
	}
	if err := json.Unmarshal(data, &mf); err != nil {
		return nil, mf, fmt.Errorf("decode manifest: %w", err)
	}
	if expectedDigest != "" {
		sum := sha256.Sum256(data)
		digest := fmt.Sprintf("%x", sum[:])
		if !strings.EqualFold(digest, strings.TrimSpace(expectedDigest)) {
			return nil, mf, fmt.Errorf("manifest digest mismatch: expected %s", expectedDigest)
		}
	}
	if err := mf.Validate(); err != nil {
		return nil, mf, fmt.Errorf("manifest validation failed: %w", err)
	}
	return data, mf, nil
}

func downloadPluginArtifact(ctx context.Context, client HTTPDoer, endpoint, authKey, userAgent, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create artifact request: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")
	if userAgent = strings.TrimSpace(userAgent); userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	if auth := strings.TrimSpace(authKey); auth != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", auth))
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download artifact: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return fmt.Errorf("download artifact: %s", message)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("prepare artifact path: %w", err)
	}

	file, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create artifact file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("write artifact: %w", err)
	}
	return nil
}

func genericInstallationUpToDate(manifestPath, artifactPath string, expectedManifest []byte, mf manifest.Manifest) (bool, error) {
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return false, err
	}
	if !bytes.Equal(manifestData, expectedManifest) {
		return false, nil
	}
	if strings.TrimSpace(mf.Package.Hash) == "" {
		if _, err := os.Stat(artifactPath); err != nil {
			return false, err
		}
		return true, nil
	}
	currentHash, err := fileHash(artifactPath)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(currentHash, mf.Package.Hash), nil
}
