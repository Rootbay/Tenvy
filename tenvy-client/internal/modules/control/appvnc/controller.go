package appvnc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

type (
	Command       = protocol.Command
	CommandResult = protocol.CommandResult
)

var errSessionReplaced = errors.New("app-vnc session replaced")

// Logger matches the agent logging contract and is satisfied by *log.Logger.
type Logger interface {
	Printf(format string, args ...interface{})
}

// Config controls the runtime behaviour of the App VNC controller.
type Config struct {
	Logger        Logger
	WorkspaceRoot string
}

// Controller processes app-vnc commands sent by the controller.
type Controller struct {
	mu            sync.Mutex
	logger        Logger
	workspaceRoot string
	session       *sessionState
}

type sessionState struct {
	id          string
	workspace   string
	process     *exec.Cmd
	application *protocol.AppVncApplicationDescriptor
	plan        *protocol.AppVncVirtualizationPlan
	settings    protocol.AppVncSessionSettings
	startedAt   time.Time
	lastBeat    time.Time
}

// NewController constructs a controller with default configuration.
func NewController() *Controller {
	return &Controller{}
}

// Update applies runtime configuration such as logger routing or workspace roots.
func (c *Controller) Update(cfg Config) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cfg.Logger != nil {
		c.logger = cfg.Logger
	}
	if root := strings.TrimSpace(cfg.WorkspaceRoot); root != "" {
		c.workspaceRoot = root
	}
}

// HandleCommand decodes the payload and dispatches the requested action.
func (c *Controller) HandleCommand(ctx context.Context, cmd Command) CommandResult {
	payload, err := decodePayload(cmd.Payload)
	if err != nil {
		return CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       err.Error(),
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	action := strings.ToLower(strings.TrimSpace(payload.Action))
	var actionErr error
	switch action {
	case "start":
		actionErr = c.start(ctx, payload)
	case "stop":
		actionErr = c.stop(payload.SessionID)
	case "configure":
		actionErr = c.configure(payload)
	case "input":
		actionErr = c.handleInput(payload)
	case "heartbeat":
		actionErr = c.heartbeat(payload)
	default:
		actionErr = fmt.Errorf("unsupported app-vnc action: %s", action)
	}

	result := CommandResult{
		CommandID:   cmd.ID,
		CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if actionErr != nil {
		result.Success = false
		result.Error = actionErr.Error()
	} else {
		result.Success = true
		result.Output = fmt.Sprintf("app-vnc %s action processed", action)
	}
	return result
}

// Shutdown terminates any active session and cleans up its workspace.
func (c *Controller) Shutdown(ctx context.Context) {
	c.mu.Lock()
	session := c.session
	c.session = nil
	c.mu.Unlock()

	if session != nil {
		c.terminateSession(ctx, session, errors.New("shutdown"))
	}
}

func (c *Controller) start(ctx context.Context, payload protocol.AppVncCommandPayload) error {
	sessionID := strings.TrimSpace(payload.SessionID)
	if sessionID == "" {
		return errors.New("missing session identifier")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session != nil && c.session.id == sessionID {
		c.applySettingsLocked(c.session, payload.Settings)
		return nil
	}

	if c.session != nil {
		prev := c.session
		c.session = nil
		c.mu.Unlock()
		c.terminateSession(ctx, prev, errSessionReplaced)
		c.mu.Lock()
	}

	if payload.Application == nil {
		return errors.New("missing application descriptor")
	}

	plan := resolveVirtualizationPlan(payload.Application, payload.Virtualization)
	settings := resolveSettings(payload.Settings)

	workspaceRoot := c.workspaceRoot
	if strings.TrimSpace(workspaceRoot) == "" {
		workspaceRoot = os.TempDir()
	}
	workspace, err := os.MkdirTemp(workspaceRoot, "tenvy-appvnc-")
	if err != nil {
		return fmt.Errorf("prepare workspace: %w", err)
	}

	cleanup := func() {
		if removeErr := os.RemoveAll(workspace); removeErr != nil {
			c.logf("app-vnc: failed to remove workspace %s: %v", workspace, removeErr)
		}
	}

	if plan != nil {
		if path := strings.TrimSpace(plan.ProfileSeed); path != "" {
			target := filepath.Join(workspace, "profile")
			if copyErr := clonePath(path, target); copyErr != nil {
				c.logf("app-vnc: profile seed copy failed (%s -> %s): %v", path, target, copyErr)
			} else {
				c.logf("app-vnc: cloned profile seed %s", target)
			}
		}
		if path := strings.TrimSpace(plan.DataRoot); path != "" {
			target := filepath.Join(workspace, "data")
			if copyErr := clonePath(path, target); copyErr != nil {
				c.logf("app-vnc: data root copy failed (%s -> %s): %v", path, target, copyErr)
			} else {
				c.logf("app-vnc: cloned data root %s", target)
			}
		}
	}

	executable, err := selectExecutable(payload.Application, plan)
	if err != nil {
		cleanup()
		return err
	}

	env := mergeEnvironment(plan)
	cmd := exec.CommandContext(ctx, executable) // #nosec G204 - path originates from static descriptor
	cmd.Dir = workspace
	if len(env) > 0 {
		cmd.Env = env
	}

	if err := cmd.Start(); err != nil {
		cleanup()
		return fmt.Errorf("launch %s: %w", executable, err)
	}

	state := &sessionState{
		id:          sessionID,
		workspace:   workspace,
		process:     cmd,
		application: payload.Application,
		plan:        plan,
		settings:    settings,
		startedAt:   time.Now(),
		lastBeat:    time.Now(),
	}
	c.session = state
	c.logf("app-vnc: session %s started (%s)", sessionID, executable)
	go c.awaitProcess(cmd, workspace)
	return nil
}

func (c *Controller) stop(sessionID string) error {
	id := strings.TrimSpace(sessionID)
	c.mu.Lock()
	session := c.session
	if session != nil && id != "" && session.id != id {
		c.mu.Unlock()
		return errors.New("session identifier mismatch")
	}
	c.session = nil
	c.mu.Unlock()

	if session == nil {
		return nil
	}

	c.terminateSession(context.Background(), session, errors.New("stop"))
	return nil
}

func (c *Controller) configure(payload protocol.AppVncCommandPayload) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return errors.New("no active session")
	}
	if id := strings.TrimSpace(payload.SessionID); id != "" && id != c.session.id {
		return errors.New("session identifier mismatch")
	}
	c.applySettingsLocked(c.session, payload.Settings)
	return nil
}

func (c *Controller) handleInput(payload protocol.AppVncCommandPayload) error {
	if len(payload.Events) == 0 {
		return nil
	}
	c.logf("app-vnc: received %d input events", len(payload.Events))
	return nil
}

func (c *Controller) heartbeat(payload protocol.AppVncCommandPayload) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return nil
	}
	if id := strings.TrimSpace(payload.SessionID); id != "" && id != c.session.id {
		return errors.New("session identifier mismatch")
	}
	c.session.lastBeat = time.Now()
	return nil
}

func (c *Controller) applySettingsLocked(session *sessionState, patch *protocol.AppVncSessionSettingsPatch) {
	if session == nil || patch == nil {
		return
	}
	if patch.Monitor != nil {
		session.settings.Monitor = strings.TrimSpace(*patch.Monitor)
	}
	if patch.Quality != nil {
		session.settings.Quality = *patch.Quality
	}
	if patch.CaptureCursor != nil {
		session.settings.CaptureCursor = *patch.CaptureCursor
	}
	if patch.ClipboardSync != nil {
		session.settings.ClipboardSync = *patch.ClipboardSync
	}
	if patch.BlockLocalInput != nil {
		session.settings.BlockLocalInput = *patch.BlockLocalInput
	}
	if patch.HeartbeatInterval != nil {
		session.settings.HeartbeatInterval = *patch.HeartbeatInterval
	}
	if patch.AppID != nil {
		session.settings.AppID = strings.TrimSpace(*patch.AppID)
	}
	if patch.WindowTitle != nil {
		session.settings.WindowTitle = strings.TrimSpace(*patch.WindowTitle)
	}
}

func (c *Controller) awaitProcess(cmd *exec.Cmd, workspace string) {
	if cmd == nil {
		return
	}
	err := cmd.Wait()
	if err != nil {
		c.logf("app-vnc: process exited with error: %v", err)
	} else {
		c.logf("app-vnc: process exited cleanly")
	}
	if workspace != "" {
		if removeErr := os.RemoveAll(workspace); removeErr != nil {
			c.logf("app-vnc: failed to remove workspace %s after exit: %v", workspace, removeErr)
		}
	}
}

func (c *Controller) terminateSession(ctx context.Context, session *sessionState, reason error) {
	if session == nil {
		return
	}
	if session.process != nil && session.process.Process != nil {
		_ = session.process.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() {
			session.process.Wait() // nolint: errcheck
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = session.process.Process.Kill()
		}
	}
	if session.workspace != "" {
		if err := os.RemoveAll(session.workspace); err != nil {
			c.logf("app-vnc: cleanup failed for %s: %v", session.workspace, err)
		}
	}
	c.logf("app-vnc: session %s stopped (%v)", session.id, reason)
}

func (c *Controller) logf(format string, args ...interface{}) {
	if c.logger != nil {
		c.logger.Printf(format, args...)
	}
}

func decodePayload(raw json.RawMessage) (protocol.AppVncCommandPayload, error) {
	var payload protocol.AppVncCommandPayload
	if len(raw) == 0 {
		return payload, errors.New("empty command payload")
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return protocol.AppVncCommandPayload{}, fmt.Errorf("invalid app-vnc payload: %w", err)
	}
	return payload, nil
}

func resolveSettings(patch *protocol.AppVncSessionSettingsPatch) protocol.AppVncSessionSettings {
	settings := protocol.AppVncSessionSettings{
		Monitor:           "Primary",
		Quality:           protocol.AppVncQualityBalanced,
		CaptureCursor:     true,
		ClipboardSync:     false,
		BlockLocalInput:   false,
		HeartbeatInterval: 30,
	}
	applySettingsPatch(&settings, patch)
	return settings
}

func applySettingsPatch(target *protocol.AppVncSessionSettings, patch *protocol.AppVncSessionSettingsPatch) {
	if target == nil || patch == nil {
		return
	}
	if patch.Monitor != nil {
		target.Monitor = strings.TrimSpace(*patch.Monitor)
	}
	if patch.Quality != nil {
		target.Quality = *patch.Quality
	}
	if patch.CaptureCursor != nil {
		target.CaptureCursor = *patch.CaptureCursor
	}
	if patch.ClipboardSync != nil {
		target.ClipboardSync = *patch.ClipboardSync
	}
	if patch.BlockLocalInput != nil {
		target.BlockLocalInput = *patch.BlockLocalInput
	}
	if patch.HeartbeatInterval != nil {
		target.HeartbeatInterval = *patch.HeartbeatInterval
	}
	if patch.AppID != nil {
		target.AppID = strings.TrimSpace(*patch.AppID)
	}
	if patch.WindowTitle != nil {
		target.WindowTitle = strings.TrimSpace(*patch.WindowTitle)
	}
}

func resolveVirtualizationPlan(
	descriptor *protocol.AppVncApplicationDescriptor,
	provided *protocol.AppVncVirtualizationPlan,
) *protocol.AppVncVirtualizationPlan {
	if descriptor == nil {
		return provided
	}
	platform := currentPlatform()
	plan := &protocol.AppVncVirtualizationPlan{Platform: platform}
	if provided != nil {
		if provided.Platform != "" {
			plan.Platform = provided.Platform
		}
		plan.ProfileSeed = strings.TrimSpace(provided.ProfileSeed)
		plan.DataRoot = strings.TrimSpace(provided.DataRoot)
		if len(provided.Environment) > 0 {
			plan.Environment = make(map[string]string, len(provided.Environment))
			for key, value := range provided.Environment {
				plan.Environment[key] = value
			}
		}
	}

	hints := descriptor.Virtualization
	if hints != nil {
		if plan.ProfileSeed == "" {
			if value, ok := hints.ProfileSeeds[plan.Platform]; ok {
				plan.ProfileSeed = value
			}
		}
		if plan.DataRoot == "" {
			if value, ok := hints.DataRoots[plan.Platform]; ok {
				plan.DataRoot = value
			}
		}
		if len(plan.Environment) == 0 {
			if env, ok := hints.Environment[plan.Platform]; ok {
				plan.Environment = make(map[string]string, len(env))
				for key, value := range env {
					plan.Environment[key] = value
				}
			}
		}
	}

	plan.ProfileSeed = strings.TrimSpace(plan.ProfileSeed)
	plan.DataRoot = strings.TrimSpace(plan.DataRoot)
	if plan.Environment != nil {
		for key, value := range plan.Environment {
			plan.Environment[key] = strings.TrimSpace(value)
		}
	}
	if plan.ProfileSeed == "" && plan.DataRoot == "" && len(plan.Environment) == 0 {
		if plan.Platform == "" {
			return nil
		}
		return &protocol.AppVncVirtualizationPlan{Platform: plan.Platform}
	}
	return plan
}

func selectExecutable(
	descriptor *protocol.AppVncApplicationDescriptor,
	plan *protocol.AppVncVirtualizationPlan,
) (string, error) {
	if descriptor == nil {
		return "", errors.New("missing application descriptor")
	}
	platform := currentPlatform()
	if plan != nil && plan.Platform != "" {
		platform = plan.Platform
	}
	execPath := ""
	if descriptor.Executable != nil {
		execPath = strings.TrimSpace(descriptor.Executable[platform])
	}
	if execPath == "" {
		return "", fmt.Errorf("no executable configured for %s", platform)
	}
	return execPath, nil
}

func mergeEnvironment(plan *protocol.AppVncVirtualizationPlan) []string {
	overrides := map[string]string{}
	if plan != nil && len(plan.Environment) > 0 {
		for key, value := range plan.Environment {
			if trimmed := strings.TrimSpace(key); trimmed != "" {
				overrides[trimmed] = value
			}
		}
	}
	if len(overrides) == 0 {
		return nil
	}
	current := map[string]string{}
	for _, entry := range os.Environ() {
		if idx := strings.Index(entry, "="); idx > 0 {
			key := entry[:idx]
			current[key] = entry[idx+1:]
		}
	}
	for key, value := range overrides {
		current[key] = value
	}
	env := make([]string, 0, len(current))
	for key, value := range current {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}

func clonePath(source, destination string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDirectory(source, destination, info.Mode())
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	return copyFile(source, destination, info.Mode())
}

func copyDirectory(source, destination string, mode fs.FileMode) error {
	if err := os.MkdirAll(destination, mode.Perm()); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(source, path)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(destination, rel)
		info, statErr := d.Info()
		if statErr != nil {
			return statErr
		}
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(source, destination string, mode fs.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func currentPlatform() protocol.AppVncPlatform {
	switch runtime.GOOS {
	case "windows":
		return protocol.AppVncPlatformWindows
	case "darwin":
		return protocol.AppVncPlatformMacOS
	default:
		return protocol.AppVncPlatformLinux
	}
}
