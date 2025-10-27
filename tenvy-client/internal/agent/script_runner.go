package agent

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	options "github.com/rootbay/tenvy-client/internal/operations/options"
	"github.com/rootbay/tenvy-client/internal/protocol"
)

const (
	scriptAutomationCommandID      = "options.script"
	scriptStatusIdle               = "idle"
	scriptStatusStaged             = "staged"
	scriptStatusScheduled          = "scheduled"
	scriptStatusWaiting            = "waiting"
	scriptStatusRunning            = "running"
	scriptStatusCompleted          = "completed"
	scriptStatusFailed             = "failed"
	scriptStatusVerificationFailed = "verification_failed"
	scriptStatusStopped            = "stopped"
)

var errScriptVerificationFailed = errors.New("script verification failed")

type scriptRunner struct {
	agent   *Agent
	manager *options.Manager

	mu      sync.Mutex
	current options.ScriptConfig
	cancel  context.CancelFunc
	runtime options.ScriptRuntimeState
}

func newScriptRunner(agent *Agent, manager *options.Manager) *scriptRunner {
	if agent == nil {
		return nil
	}
	runner := &scriptRunner{
		agent:   agent,
		manager: manager,
		runtime: options.ScriptRuntimeState{Status: scriptStatusIdle},
	}
	return runner
}

func (r *scriptRunner) Apply(ctx context.Context, cfg options.ScriptConfig) {
	if r == nil {
		return
	}
	clone := cloneScriptConfig(cfg)

	r.mu.Lock()
	previous := r.current
	if scriptConfigEqual(previous, clone) {
		r.mu.Unlock()
		return
	}
	cancel := r.cancel
	r.cancel = nil
	fileChanged := !scriptFilesEqual(previous.File, clone.File)
	r.current = clone
	if fileChanged {
		r.runtime.Runs = 0
	}
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if clone.File == nil || strings.TrimSpace(clone.File.Path) == "" {
		r.setIdle(scriptStatusIdle)
		return
	}

	if ctx == nil {
		ctx = context.Background()
	}

	runCtx, cancel := context.WithCancel(ctx)
	r.mu.Lock()
	r.cancel = cancel
	r.mu.Unlock()

	go r.runLoop(runCtx, clone)
}

func (r *scriptRunner) Stop() {
	if r == nil {
		return
	}

	r.mu.Lock()
	cancel := r.cancel
	r.cancel = nil
	r.current = options.ScriptConfig{}
	r.runtime.Active = false
	r.runtime.Status = scriptStatusStopped
	r.runtime.LastError = ""
	r.runtime.HasExitCode = false
	r.runtime.LastCompletedAt = time.Now()
	stateCopy := r.runtime
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	r.updateManagerRuntime(stateCopy)
	if r.agent != nil && r.agent.logger != nil {
		r.agent.logger.Println("script automation stopped")
	}
}

func (r *scriptRunner) Status() options.ScriptRuntimeState {
	if r == nil {
		return options.ScriptRuntimeState{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runtime
}

func (r *scriptRunner) runLoop(ctx context.Context, cfg options.ScriptConfig) {
	defer func() {
		if ctx.Err() != nil {
			r.setIdle(scriptStatusStopped)
		}
	}()

	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	loop := cfg.Loop || mode == "looped"
	delaySeconds := cfg.DelaySeconds
	if delaySeconds < 0 {
		delaySeconds = 0
	}
	delay := time.Duration(delaySeconds) * time.Second

	firstRun := true

	for {
		if ctx.Err() != nil {
			return
		}

		if firstRun {
			if mode == "delayed" && delay > 0 {
				r.markScheduled(delay)
				if err := sleepContext(ctx, delay); err != nil {
					return
				}
			} else {
				r.markScheduled(0)
			}
		} else if loop {
			if delay > 0 {
				r.markWaiting(delay)
				if err := sleepContext(ctx, delay); err != nil {
					return
				}
			} else {
				r.markWaiting(0)
			}
		}

		if ctx.Err() != nil {
			return
		}

		runNumber, startedAt := r.markRunStarted()
		if r.agent != nil && r.agent.logger != nil {
			r.agent.logger.Printf("script automation run %d started: %s", runNumber, scriptDisplayName(cfg.File))
		}
		output, exitCode, execErr := r.execute(ctx, cfg.File)

		if ctx.Err() != nil {
			return
		}

		duration := time.Since(startedAt)

		if execErr != nil {
			if errors.Is(execErr, errScriptVerificationFailed) {
				r.markVerificationFailed(execErr)
				r.reportFailure(runNumber, cfg, -1, duration, output, execErr)
				return
			}

			r.markRunFinished(exitCode, execErr)
			r.reportFailure(runNumber, cfg, exitCode, duration, output, execErr)
			if !loop {
				return
			}
			firstRun = false
			continue
		}

		r.markRunFinished(exitCode, nil)
		r.reportSuccess(runNumber, cfg, exitCode, duration, output)
		if !loop {
			return
		}
		firstRun = false
	}
}

func (r *scriptRunner) markScheduled(delay time.Duration) {
	r.mu.Lock()
	r.runtime.Active = false
	r.runtime.Status = scriptStatusScheduled
	r.runtime.LastError = ""
	stateCopy := r.runtime
	r.mu.Unlock()
	r.updateManagerRuntime(stateCopy)
}

func (r *scriptRunner) markWaiting(delay time.Duration) {
	r.mu.Lock()
	r.runtime.Active = false
	r.runtime.Status = scriptStatusWaiting
	stateCopy := r.runtime
	r.mu.Unlock()
	r.updateManagerRuntime(stateCopy)
}

func (r *scriptRunner) markRunStarted() (int64, time.Time) {
	now := time.Now()
	r.mu.Lock()
	r.runtime.Active = true
	r.runtime.Status = scriptStatusRunning
	r.runtime.LastStartedAt = now
	r.runtime.LastError = ""
	r.runtime.HasExitCode = false
	r.runtime.Runs++
	runNumber := r.runtime.Runs
	stateCopy := r.runtime
	r.mu.Unlock()
	r.updateManagerRuntime(stateCopy)
	return runNumber, now
}

func (r *scriptRunner) markRunFinished(exitCode int, execErr error) {
	now := time.Now()
	r.mu.Lock()
	r.runtime.Active = false
	r.runtime.LastCompletedAt = now
	if exitCode >= 0 {
		r.runtime.LastExitCode = exitCode
		r.runtime.HasExitCode = true
	} else {
		r.runtime.HasExitCode = false
	}
	if execErr != nil {
		r.runtime.Status = scriptStatusFailed
		r.runtime.LastError = execErr.Error()
	} else {
		r.runtime.Status = scriptStatusCompleted
		r.runtime.LastError = ""
	}
	stateCopy := r.runtime
	r.mu.Unlock()
	r.updateManagerRuntime(stateCopy)
}

func (r *scriptRunner) markVerificationFailed(err error) {
	now := time.Now()
	r.mu.Lock()
	r.runtime.Active = false
	r.runtime.Status = scriptStatusVerificationFailed
	r.runtime.LastCompletedAt = now
	r.runtime.LastError = err.Error()
	r.runtime.HasExitCode = false
	stateCopy := r.runtime
	r.mu.Unlock()
	r.updateManagerRuntime(stateCopy)
}

func (r *scriptRunner) setIdle(status string) {
	if status == "" {
		status = scriptStatusIdle
	}
	r.mu.Lock()
	r.runtime.Active = false
	r.runtime.Status = status
	if status == scriptStatusIdle {
		r.runtime.LastError = ""
		r.runtime.HasExitCode = false
	}
	stateCopy := r.runtime
	r.mu.Unlock()
	r.updateManagerRuntime(stateCopy)
}

func (r *scriptRunner) execute(ctx context.Context, file *options.ScriptFile) (string, int, error) {
	if err := r.verifyScript(file); err != nil {
		return "", -1, fmt.Errorf("%w: %v", errScriptVerificationFailed, err)
	}

	cmd, err := r.buildCommand(ctx, file)
	if err != nil {
		return "", -1, err
	}

	output, err := cmd.CombinedOutput()
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else if exitCode == 0 {
			exitCode = -1
		}
	}

	return string(output), exitCode, err
}

func (r *scriptRunner) verifyScript(file *options.ScriptFile) error {
	if file == nil {
		return fmt.Errorf("script file not staged")
	}
	path := strings.TrimSpace(file.Path)
	if path == "" {
		return fmt.Errorf("script path missing")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("access script: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("script path is a directory")
	}

	handle, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open script: %w", err)
	}
	defer handle.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, handle); err != nil {
		return fmt.Errorf("read script: %w", err)
	}
	computed := fmt.Sprintf("%x", hasher.Sum(nil))
	expected := strings.TrimSpace(file.Checksum)
	if expected != "" && !strings.EqualFold(expected, computed) {
		return fmt.Errorf("checksum mismatch")
	}
	if file.Size > 0 && info.Size() != file.Size {
		return fmt.Errorf("size mismatch (expected %d, got %d)", file.Size, info.Size())
	}
	return nil
}

func (r *scriptRunner) buildCommand(ctx context.Context, file *options.ScriptFile) (*exec.Cmd, error) {
	if file == nil {
		return nil, fmt.Errorf("script file not staged")
	}
	path := strings.TrimSpace(file.Path)
	if path == "" {
		return nil, fmt.Errorf("script path missing")
	}
	var cmd *exec.Cmd
	ext := strings.ToLower(filepath.Ext(path))
	switch runtime.GOOS {
	case "windows":
		if ext == ".ps1" || strings.Contains(strings.ToLower(file.Type), "powershell") {
			cmd = exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", path)
		} else {
			cmd = exec.CommandContext(ctx, "cmd.exe", "/C", path)
		}
	default:
		cmd = exec.CommandContext(ctx, "/bin/sh", path)
	}
	cmd.Dir = filepath.Dir(path)
	return cmd, nil
}

func (r *scriptRunner) reportSuccess(run int64, cfg options.ScriptConfig, exitCode int, duration time.Duration, output string) {
	if r == nil || r.agent == nil {
		return
	}

	name := scriptDisplayName(cfg.File)
	summary := fmt.Sprintf("Script %s completed (run %d, exit=%d, duration=%s)", name, run, exitCode, formatDuration(duration))

	trimmed := strings.TrimSpace(output)
	if trimmed != "" {
		summary = summary + "\n" + truncateMultiline(trimmed, 4096)
	}

	result := protocol.CommandResult{
		CommandID:   scriptAutomationCommandID,
		Success:     true,
		Output:      summary,
		CompletedAt: timestampNow(),
	}
	r.agent.enqueueResult(result)
	if r.agent.logger != nil {
		r.agent.logger.Printf("script automation run %d completed: exit=%d", run, exitCode)
	}
}

func (r *scriptRunner) reportFailure(run int64, cfg options.ScriptConfig, exitCode int, duration time.Duration, output string, execErr error) {
	if r == nil || r.agent == nil {
		return
	}

	name := scriptDisplayName(cfg.File)
	base := fmt.Sprintf("Script %s failed (run %d", name, run)
	if exitCode >= 0 {
		base = fmt.Sprintf("%s, exit=%d", base, exitCode)
	}
	base += ")"
	if duration > 0 {
		base = fmt.Sprintf("%s after %s", base, formatDuration(duration))
	}
	if execErr != nil {
		base = fmt.Sprintf("%s: %v", base, execErr)
	}

	trimmed := strings.TrimSpace(output)
	if trimmed != "" {
		base = base + "\n" + truncateMultiline(trimmed, 4096)
	}

	result := protocol.CommandResult{
		CommandID:   scriptAutomationCommandID,
		Success:     false,
		Error:       base,
		CompletedAt: timestampNow(),
	}
	r.agent.enqueueResult(result)
	if r.agent.logger != nil {
		r.agent.logger.Printf("script automation run %d failed: %v", run, execErr)
	}
}

func (r *scriptRunner) updateManagerRuntime(state options.ScriptRuntimeState) {
	if r.manager != nil {
		r.manager.SetScriptRuntime(state)
	}
}

func scriptDisplayName(file *options.ScriptFile) string {
	if file == nil {
		return "(none)"
	}
	if name := strings.TrimSpace(file.Name); name != "" {
		return name
	}
	if path := strings.TrimSpace(file.Path); path != "" {
		return filepath.Base(path)
	}
	return "script"
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	if d < time.Millisecond {
		return d.String()
	}
	if d < time.Second {
		return d.Round(100 * time.Microsecond).String()
	}
	return d.Round(time.Millisecond).String()
}

func truncateMultiline(input string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(input)
	if len(runes) <= limit {
		return input
	}
	return string(runes[:limit]) + "…"
}

func scriptConfigEqual(a, b options.ScriptConfig) bool {
	if !scriptFilesEqual(a.File, b.File) {
		return false
	}
	if strings.TrimSpace(strings.ToLower(a.Mode)) != strings.TrimSpace(strings.ToLower(b.Mode)) {
		return false
	}
	if a.Loop != b.Loop {
		return false
	}
	if a.DelaySeconds != b.DelaySeconds {
		return false
	}
	return true
}

func scriptFilesEqual(a, b *options.ScriptFile) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	}
	return a.Name == b.Name &&
		a.Path == b.Path &&
		a.Checksum == b.Checksum &&
		a.Type == b.Type &&
		a.Size == b.Size
}

func cloneScriptConfig(cfg options.ScriptConfig) options.ScriptConfig {
	clone := cfg
	if cfg.File != nil {
		fileCopy := *cfg.File
		clone.File = &fileCopy
	}
	return clone
}

func scriptConfigFingerprint(cfg options.ScriptConfig) string {
	var builder strings.Builder
	builder.WriteString(strings.ToLower(strings.TrimSpace(cfg.Mode)))
	builder.WriteRune('|')
	if cfg.Loop {
		builder.WriteString("loop")
	}
	builder.WriteRune('|')
	builder.WriteString(fmt.Sprintf("%d", cfg.DelaySeconds))
	builder.WriteRune('|')
	if cfg.File != nil {
		builder.WriteString(strings.TrimSpace(cfg.File.Path))
		builder.WriteRune('|')
		builder.WriteString(strings.TrimSpace(cfg.File.Checksum))
		builder.WriteRune('|')
		builder.WriteString(strings.TrimSpace(cfg.File.Name))
	}
	return builder.String()
}

func (a *Agent) monitorScriptAutomation(ctx context.Context) {
	if a == nil || a.options == nil || a.scriptRunner == nil {
		return
	}

	snapshot := a.options.Snapshot()
	fingerprint := scriptConfigFingerprint(snapshot.Script)
	a.scriptRunner.Apply(ctx, snapshot.Script)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.scriptRunner.Stop()
			return
		case <-ticker.C:
			state := a.options.Snapshot()
			next := scriptConfigFingerprint(state.Script)
			if next != fingerprint {
				fingerprint = next
				a.scriptRunner.Apply(ctx, state.Script)
			}
		}
	}
}

func (a *Agent) scriptAutomationSummary() string {
	if a == nil || a.scriptRunner == nil {
		return "Script automation unavailable"
	}
	state := a.scriptRunner.Status()
	return describeScriptStatus(state)
}

func describeScriptStatus(state options.ScriptRuntimeState) string {
	switch state.Status {
	case scriptStatusRunning:
		if !state.LastStartedAt.IsZero() {
			return fmt.Sprintf("Script automation running (started %s ago)", humanizeSince(state.LastStartedAt))
		}
		return "Script automation running"
	case scriptStatusWaiting:
		runDescriptor := ""
		if state.Runs > 0 {
			runDescriptor = fmt.Sprintf(" (%d run%s completed)", state.Runs, pluralize(state.Runs))
		}
		return "Script automation waiting for next run" + runDescriptor
	case scriptStatusScheduled:
		return "Script automation scheduled"
	case scriptStatusCompleted:
		if state.HasExitCode {
			if !state.LastCompletedAt.IsZero() {
				return fmt.Sprintf("Script automation completed %s ago (exit=%d)", humanizeSince(state.LastCompletedAt), state.LastExitCode)
			}
			return fmt.Sprintf("Script automation completed (exit=%d)", state.LastExitCode)
		}
		return "Script automation completed"
	case scriptStatusFailed:
		summary := "Script automation failed"
		if state.HasExitCode {
			summary = fmt.Sprintf("%s (exit=%d)", summary, state.LastExitCode)
		}
		if state.LastError != "" {
			summary = fmt.Sprintf("%s: %s", summary, state.LastError)
		}
		return summary
	case scriptStatusVerificationFailed:
		if state.LastError != "" {
			return fmt.Sprintf("Script automation verification failed: %s", state.LastError)
		}
		return "Script automation verification failed"
	case scriptStatusStopped:
		return "Script automation stopped"
	case scriptStatusStaged:
		return "Script automation staged and awaiting execution"
	case "", scriptStatusIdle:
		return "Script automation idle"
	default:
		return fmt.Sprintf("Script automation status: %s", state.Status)
	}
}

func humanizeSince(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return formatDuration(time.Since(t))
}

func pluralize(value int64) string {
	if value == 1 {
		return ""
	}
	return "s"
}
