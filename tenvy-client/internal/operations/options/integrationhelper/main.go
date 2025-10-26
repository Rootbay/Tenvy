package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rootbay/tenvy-client/internal/operations/options"
	"github.com/rootbay/tenvy-client/internal/protocol"
)

type stubPlatformService struct{}

func (stubPlatformService) Execute(
	ctx context.Context,
	operation string,
	metadata map[string]any,
	state options.State,
) (string, error) {
	switch operation {
	case "defender-exclusion":
		enabled, _ := metadata["enabled"].(bool)
		if enabled {
			return "Stub defender exclusion enabled", nil
		}
		return "Stub defender exclusion disabled", nil
	case "windows-update":
		enabled, _ := metadata["enabled"].(bool)
		if enabled {
			return "Stub Windows Update enabled", nil
		}
		return "Stub Windows Update disabled", nil
	case "sound-playback":
		enabled, _ := metadata["enabled"].(bool)
		if enabled {
			return fmt.Sprintf("Stub playback restored to %d%%", state.SoundVolume), nil
		}
		return "Stub playback muted", nil
	case "sound-volume":
		if volume, ok := metadata["volume"].(int); ok {
			return fmt.Sprintf("Stub volume set to %d%%", volume), nil
		}
		return "Stub volume adjusted", nil
	default:
		return fmt.Sprintf("Stub operation %s applied", operation), nil
	}
}

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fail(fmt.Errorf("read input: %w", err))
	}

	var cmd protocol.Command
	if err := json.Unmarshal(data, &cmd); err != nil {
		fail(fmt.Errorf("decode command: %w", err))
	}

	if !strings.EqualFold(strings.TrimSpace(cmd.Name), "tool-activation") {
		fail(fmt.Errorf("unsupported command: %s", cmd.Name))
	}

	var payload protocol.ToolActivationCommandPayload
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		fail(fmt.Errorf("decode payload: %w", err))
	}

	action := strings.TrimSpace(payload.Action)
	if action == "" {
		fail(fmt.Errorf("missing tool action"))
	}
	lower := strings.ToLower(action)
	if !strings.HasPrefix(lower, "operation:") {
		fail(fmt.Errorf("unsupported action: %s", action))
	}
	operation := strings.TrimSpace(action[len("operation:"):])
	if operation == "" {
		fail(fmt.Errorf("missing operation name"))
	}

	manager := options.NewManager(options.ManagerOptions{})
	manager.SetPlatformService(stubPlatformService{})

	summary, opErr := manager.ApplyOperation(context.Background(), operation, payload.Metadata, nil)

	result := protocol.CommandResult{
		CommandID:   cmd.ID,
		CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if opErr != nil {
		result.Success = false
		result.Error = opErr.Error()
	} else {
		result.Success = true
		result.Output = summary
	}

	output, err := json.Marshal(result)
	if err != nil {
		fail(fmt.Errorf("encode result: %w", err))
	}
	if _, err := os.Stdout.Write(output); err != nil {
		fail(fmt.Errorf("write result: %w", err))
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "%v\n", err)
	os.Exit(1)
}
