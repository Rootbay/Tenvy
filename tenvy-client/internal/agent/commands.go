package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/rootbay/tenvy-client/internal/platform"
	"github.com/rootbay/tenvy-client/internal/protocol"
)

type shellExecutionOptions struct {
	workingDirectory string
	environment      map[string]string
}

func (a *Agent) processCommands(ctx context.Context, commands []protocol.Command) {
	for _, cmd := range commands {
		if err := ctx.Err(); err != nil {
			a.enqueueResult(newFailureResult(cmd.ID, "agent shutting down"))
			continue
		}

		a.logger.Printf("executing command %s (%s)", cmd.ID, cmd.Name)
		result := a.executeCommand(ctx, cmd)
		a.enqueueResult(result)
	}
}

func (a *Agent) executeCommand(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
	if a.commands == nil {
		return newFailureResult(cmd.ID, "command router not initialized")
	}
	return a.commands.dispatch(ctx, a, cmd)
}

func pingCommandHandler(_ context.Context, _ *Agent, cmd protocol.Command) protocol.CommandResult {
	var payload protocol.PingCommandPayload
	_ = json.Unmarshal(cmd.Payload, &payload)
	response := "pong"
	if strings.TrimSpace(payload.Message) != "" {
		response = payload.Message
	}
	return newSuccessResult(cmd.ID, response)
}

func shellCommandHandler(ctx context.Context, agent *Agent, cmd protocol.Command) protocol.CommandResult {
	if agent == nil {
		return newFailureResult(cmd.ID, "shell command requires agent context")
	}
	var payload protocol.ShellCommandPayload
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		return newFailureResult(cmd.ID, fmt.Sprintf("invalid shell payload: %v", err))
	}

	if strings.TrimSpace(payload.Command) == "" {
		return newFailureResult(cmd.ID, "missing command")
	}

	if payload.Elevated && !platform.CurrentUserIsElevated() {
		return newFailureResult(cmd.ID, "elevated execution requested but agent is not running with sufficient privileges")
	}

	workingDirectory, err := normalizeWorkingDirectory(payload.WorkingDirectory)
	if err != nil {
		return newFailureResult(cmd.ID, err.Error())
	}

	timeout := agent.shellTimeout()
	if payload.TimeoutSeconds > 0 {
		timeout = time.Duration(payload.TimeoutSeconds) * time.Second
	}

	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	output, err := runShell(commandCtx, payload.Command, shellExecutionOptions{
		workingDirectory: workingDirectory,
		environment:      payload.Environment,
	})
	if err != nil {
		return newDetailedResult(cmd.ID, false, string(output), err.Error())
	}
	return newDetailedResult(cmd.ID, true, string(output), "")
}

func openURLCommandHandler(_ context.Context, _ *Agent, cmd protocol.Command) protocol.CommandResult {
	var payload protocol.OpenURLCommandPayload
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		return newFailureResult(cmd.ID, fmt.Sprintf("invalid open-url payload: %v", err))
	}

	trimmed := strings.TrimSpace(payload.URL)
	if trimmed == "" {
		return newFailureResult(cmd.ID, "missing url")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return newFailureResult(cmd.ID, fmt.Sprintf("invalid url: %v", err))
	}

	if !parsed.IsAbs() || parsed.Host == "" {
		return newFailureResult(cmd.ID, "url must be absolute")
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return newFailureResult(cmd.ID, fmt.Sprintf("unsupported url scheme: %s", parsed.Scheme))
	}

	normalized := parsed.String()
	if err := openURLInBrowser(normalized); err != nil {
		return newFailureResult(cmd.ID, fmt.Sprintf("failed to open url: %v", err))
	}

	return newSuccessResult(cmd.ID, fmt.Sprintf("opened %s", normalized))
}

func normalizeWorkingDirectory(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	resolved := trimmed
	if !filepath.IsAbs(resolved) {
		absPath, err := filepath.Abs(resolved)
		if err != nil {
			return "", fmt.Errorf("resolve working directory: %w", err)
		}
		resolved = absPath
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("invalid working directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("working directory is not a directory: %s", resolved)
	}

	return resolved, nil
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
	return mergeEnvironmentsWithComparer(base, overrides, runtime.GOOS == "windows")
}

func mergeEnvironmentsWithComparer(base []string, overrides map[string]string, caseInsensitive bool) []string {
	type overrideEntry struct {
		normalized string
		key        string
		value      string
	}

	normalizeKey := func(key string) string {
		if caseInsensitive {
			return strings.ToLower(key)
		}
		return key
	}

	overridesByKey := make(map[string]overrideEntry, len(overrides))
	for key, value := range overrides {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		normalizedKey := normalizeKey(trimmedKey)
		overridesByKey[normalizedKey] = overrideEntry{
			normalized: normalizedKey,
			key:        trimmedKey,
			value:      value,
		}
	}

	env := make([]string, 0, len(base)+len(overridesByKey))
	seenBaseKeys := make(map[string]struct{}, len(base))
	pendingOverrides := make([]overrideEntry, 0, len(overridesByKey))

	for _, kv := range base {
		keyPortion := kv
		if idx := strings.IndexRune(kv, '='); idx >= 0 {
			keyPortion = kv[:idx]
		}
		trimmedKey := strings.TrimSpace(keyPortion)
		normalizedKey := normalizeKey(trimmedKey)

		if normalizedKey == "" {
			env = append(env, kv)
			continue
		}

		if _, seen := seenBaseKeys[normalizedKey]; seen {
			continue
		}
		seenBaseKeys[normalizedKey] = struct{}{}

		if entry, overridden := overridesByKey[normalizedKey]; overridden {
			pendingOverrides = append(pendingOverrides, entry)
			delete(overridesByKey, normalizedKey)
			continue
		}

		env = append(env, kv)
	}

	for _, entry := range pendingOverrides {
		env = append(env, fmt.Sprintf("%s=%s", entry.key, entry.value))
	}

	if len(overridesByKey) > 0 {
		remaining := make([]overrideEntry, 0, len(overridesByKey))
		for _, entry := range overridesByKey {
			remaining = append(remaining, entry)
		}
		sort.SliceStable(remaining, func(i, j int) bool {
			return remaining[i].key < remaining[j].key
		})
		for _, entry := range remaining {
			env = append(env, fmt.Sprintf("%s=%s", entry.key, entry.value))
		}
	}

	return env
}

func newDetailedResult(cmdID string, success bool, output, errMsg string) protocol.CommandResult {
	return protocol.CommandResult{
		CommandID:   cmdID,
		Success:     success,
		Output:      output,
		Error:       errMsg,
		CompletedAt: timestampNow(),
	}
}

func newFailureResult(cmdID, errMsg string) protocol.CommandResult {
	return newDetailedResult(cmdID, false, "", errMsg)
}

func newSuccessResult(cmdID, output string) protocol.CommandResult {
	return newDetailedResult(cmdID, true, output, "")
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

func (a *Agent) shellTimeout() time.Duration {
	if a.timing.ShellTimeout > 0 {
		return a.timing.ShellTimeout
	}
	return defaultShellTimeout
}
