//go:build darwin

package options

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type darwinPlatformService struct {
	osaPath string
}

func newPlatformService() PlatformService {
	path, _ := exec.LookPath("osascript")
	return &darwinPlatformService{osaPath: path}
}

func (s *darwinPlatformService) Execute(ctx context.Context, operation string, metadata map[string]any, state State) (string, error) {
	switch operation {
	case "sound-playback":
		enabled := state.SoundPlayback
		if v, ok := metadata["enabled"].(bool); ok {
			enabled = v
		}
		return s.toggleSound(ctx, enabled)
	case "sound-volume":
		volume := state.SoundVolume
		if raw, ok := metadata["volume"]; ok {
			switch v := raw.(type) {
			case int:
				volume = v
			case int64:
				volume = int(v)
			case float64:
				volume = int(v)
			}
		}
		if volume < 0 {
			volume = 0
		}
		if volume > 100 {
			volume = 100
		}
		return s.setVolume(ctx, volume)
	default:
		return "", nil
	}
}

func (s *darwinPlatformService) toggleSound(ctx context.Context, enabled bool) (string, error) {
	if s.osaPath == "" {
		return "", nil
	}
	muteExpr := "true"
	summary := "Muted system playback"
	if enabled {
		muteExpr = "false"
		summary = "Unmuted system playback"
	}
	script := fmt.Sprintf("set volume with output muted %s", muteExpr)
	cmd := exec.CommandContext(ctx, s.osaPath, "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("toggle playback: %v", strings.TrimSpace(string(output)))
	}
	return summary, nil
}

func (s *darwinPlatformService) setVolume(ctx context.Context, volume int) (string, error) {
	if s.osaPath == "" {
		return "", nil
	}
	script := fmt.Sprintf("set volume output volume %d", volume)
	cmd := exec.CommandContext(ctx, s.osaPath, "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("set volume: %v", strings.TrimSpace(string(output)))
	}
	return fmt.Sprintf("Set system volume to %d%%", volume), nil
}
