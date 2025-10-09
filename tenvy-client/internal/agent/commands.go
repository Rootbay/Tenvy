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
		if ctx.Err() != nil {
			a.enqueueResult(protocol.CommandResult{
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

func (a *Agent) executeCommand(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
	switch cmd.Name {
	case "ping":
		return handlePingCommand(cmd)
	case "shell":
		return a.handleShellCommand(ctx, cmd)
	case "open-url":
		return handleOpenURLCommand(cmd)
	default:
		if a.modules != nil {
			if handled, result := a.modules.HandleCommand(ctx, cmd); handled {
				return result
			}
		}
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       fmt.Sprintf("unsupported command: %s", cmd.Name),
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}
}

func handlePingCommand(cmd protocol.Command) protocol.CommandResult {
	var payload protocol.PingCommandPayload
	_ = json.Unmarshal(cmd.Payload, &payload)
	response := "pong"
	if strings.TrimSpace(payload.Message) != "" {
		response = payload.Message
	}
	return protocol.CommandResult{
		CommandID:   cmd.ID,
		Success:     true,
		Output:      response,
		CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func (a *Agent) handleShellCommand(ctx context.Context, cmd protocol.Command) protocol.CommandResult {
	var payload protocol.ShellCommandPayload
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       fmt.Sprintf("invalid shell payload: %v", err),
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	if strings.TrimSpace(payload.Command) == "" {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "missing command",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	if payload.Elevated && !platform.CurrentUserIsElevated() {
		return protocol.CommandResult{
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
			return protocol.CommandResult{
				CommandID:   cmd.ID,
				Success:     false,
				Error:       fmt.Sprintf("invalid working directory: %v", err),
				CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
		}
		if !info.IsDir() {
			return protocol.CommandResult{
				CommandID:   cmd.ID,
				Success:     false,
				Error:       fmt.Sprintf("working directory is not a directory: %s", workingDirectory),
				CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
		}
	}

	timeout := a.shellTimeout()
	if payload.TimeoutSeconds > 0 {
		timeout = time.Duration(payload.TimeoutSeconds) * time.Second
	}

	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	output, err := runShell(commandCtx, payload.Command, shellExecutionOptions{
		workingDirectory: workingDirectory,
		environment:      payload.Environment,
	})
	result := protocol.CommandResult{
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

func handleOpenURLCommand(cmd protocol.Command) protocol.CommandResult {
	var payload protocol.OpenURLCommandPayload
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       fmt.Sprintf("invalid open-url payload: %v", err),
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	trimmed := strings.TrimSpace(payload.URL)
	if trimmed == "" {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "missing url",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       fmt.Sprintf("invalid url: %v", err),
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	if !parsed.IsAbs() || parsed.Host == "" {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       "url must be absolute",
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       fmt.Sprintf("unsupported url scheme: %s", parsed.Scheme),
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	normalized := parsed.String()
	if err := openURLInBrowser(normalized); err != nil {
		return protocol.CommandResult{
			CommandID:   cmd.ID,
			Success:     false,
			Error:       fmt.Sprintf("failed to open url: %v", err),
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	return protocol.CommandResult{
		CommandID:   cmd.ID,
		Success:     true,
		Output:      fmt.Sprintf("opened %s", normalized),
		CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
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

	overrideIndex := make(map[string]int, len(overrides))
	overrideEntries := make([]overrideEntry, 0, len(overrides))
	normalizeKey := func(key string) string {
		if caseInsensitive {
			return strings.ToLower(key)
		}
		return key
	}

	for key, value := range overrides {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		normalizedKey := normalizeKey(trimmedKey)
		entry := overrideEntry{normalized: normalizedKey, key: trimmedKey, value: value}
		if idx, exists := overrideIndex[normalizedKey]; exists {
			overrideEntries[idx] = entry
			continue
		}
		overrideIndex[normalizedKey] = len(overrideEntries)
		overrideEntries = append(overrideEntries, entry)
	}

	sort.SliceStable(overrideEntries, func(i, j int) bool {
		return overrideEntries[i].key < overrideEntries[j].key
	})

	env := make([]string, 0, len(base)+len(overrideEntries))
	seenKeys := make(map[string]struct{}, len(base))

	for _, kv := range base {
		keyPortion := kv
		if idx := strings.IndexRune(kv, '='); idx >= 0 {
			keyPortion = kv[:idx]
		}
		trimmedKey := strings.TrimSpace(keyPortion)
		normalizedKey := normalizeKey(trimmedKey)
		if normalizedKey != "" {
			if _, overridden := overrideIndex[normalizedKey]; overridden {
				continue
			}
			if _, seen := seenKeys[normalizedKey]; seen {
				continue
			}
			seenKeys[normalizedKey] = struct{}{}
		}
		env = append(env, kv)
	}

	for _, entry := range overrideEntries {
		env = append(env, fmt.Sprintf("%s=%s", entry.key, entry.value))
	}

	return env
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
