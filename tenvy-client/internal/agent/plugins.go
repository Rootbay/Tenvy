package agent

import (
	"os"
	"path/filepath"
	"strings"
)

func defaultPluginRoot(pref BuildPreferences) string {
	installPath := strings.TrimSpace(pref.InstallPath)
	if installPath != "" {
		cleaned := filepath.Clean(installPath)
		info, err := os.Stat(cleaned)
		if err == nil && info.IsDir() {
			return filepath.Join(cleaned, "plugins")
		}
		return filepath.Join(filepath.Dir(cleaned), "plugins")
	}

	if exe, err := os.Executable(); err == nil {
		base := filepath.Dir(exe)
		return filepath.Join(base, "plugins")
	}

	return filepath.Join(os.TempDir(), "tenvy", "plugins")
}
