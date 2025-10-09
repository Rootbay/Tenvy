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
	case "remote-desktop":
		if a.remoteDesktop == nil {
			return protocol.CommandResult{
				CommandID:   cmd.ID,
				Success:     false,
				Error:       "remote desktop subsystem not initialized",
				CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
		}
		return a.remoteDesktop.HandleCommand(ctx, cmd)
	case "audio-control":
		if a.audioBridge == nil {
			return protocol.CommandResult{
				CommandID:   cmd.ID,
				Success:     false,
				Error:       "audio subsystem not initialized",
				CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
		}
		return a.audioBridge.HandleCommand(ctx, cmd)
	case "system-info":
		if a.systemInfo == nil {
			return protocol.CommandResult{
				CommandID:   cmd.ID,
				Success:     false,
				Error:       "system information subsystem not initialized",
				CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
		}
		return a.systemInfo.HandleCommand(ctx, cmd)
	case "clipboard":
		if a.clipboard == nil {
			return protocol.CommandResult{
				CommandID:   cmd.ID,
				Success:     false,
				Error:       "clipboard subsystem not initialized",
				CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
		}
		return a.clipboard.HandleCommand(ctx, cmd)
	case "open-url":
		return handleOpenURLCommand(cmd)
	default:
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
