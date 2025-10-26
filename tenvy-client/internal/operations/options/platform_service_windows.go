//go:build windows

package options

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	winoptions "github.com/rootbay/tenvy-client/internal/platform/windows/options"
)

type windowsPlatformService struct{}

func newPlatformService() PlatformService {
	return &windowsPlatformService{}
}

func (s *windowsPlatformService) Execute(
	ctx context.Context,
	operation string,
	metadata map[string]any,
	state State,
) (string, error) {
	switch operation {
	case "defender-exclusion":
		enabled := false
		if v, ok := metadata["enabled"].(bool); ok {
			enabled = v
		}

		executable, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("resolve executable path: %w", err)
		}
		executable = filepath.Clean(executable)

		if enabled {
			if err := winoptions.EnsureProcessExclusion(ctx, executable); err != nil {
				return "", err
			}
			return fmt.Sprintf("Added Defender process exclusion for %s", executable), nil
		}

		if err := winoptions.RemoveProcessExclusion(ctx, executable); err != nil {
			return "", err
		}
		return fmt.Sprintf("Removed Defender process exclusion for %s", executable), nil

	case "windows-update":
		enabled := false
		if v, ok := metadata["enabled"].(bool); ok {
			enabled = v
		}
		if err := winoptions.ConfigureAutomaticUpdates(ctx, enabled); err != nil {
			return "", err
		}
		if enabled {
			return "Enabled Windows Update automatic updates", nil
		}
		return "Disabled Windows Update automatic updates", nil

	case "sound-playback":
		enabled := false
		if v, ok := metadata["enabled"].(bool); ok {
			enabled = v
		}

		targetVolume := state.SoundVolume
		if targetVolume < 0 {
			targetVolume = 0
		}
		if targetVolume > 100 {
			targetVolume = 100
		}

		scalar := 0.0
		if enabled {
			scalar = float64(targetVolume) / 100.0
		}

		if err := winoptions.SetMasterVolumeScalar(scalar); err != nil {
			return "", err
		}
		if enabled {
			return fmt.Sprintf("Restored system playback volume to %d%%", targetVolume), nil
		}
		return "Muted system playback", nil

	case "sound-volume":
		volume := state.SoundVolume
		if raw, ok := metadata["volume"]; ok {
			switch v := raw.(type) {
			case int:
				volume = v
			case int64:
				volume = int(v)
			case uint64:
				volume = int(v)
			case float64:
				volume = int(v)
			case float32:
				volume = int(v)
			}
		}

		if volume < 0 {
			volume = 0
		}
		if volume > 100 {
			volume = 100
		}

		if err := winoptions.SetMasterVolumeScalar(float64(volume) / 100.0); err != nil {
			return "", err
		}
		return fmt.Sprintf("Set system volume to %d%%", volume), nil
	default:
		return "", nil
	}
}
