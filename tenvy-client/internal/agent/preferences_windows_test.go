//go:build windows

package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegisterStartupPreferenceWindows(t *testing.T) {
	tmp := t.TempDir()
	runFile := filepath.Join(tmp, "run.txt")
	t.Setenv("TENVY_WINDOWS_RUN_FILE", runFile)

	target := `C:\\Program Files\\Tenvy\\tenvy.exe`
	pref := BuildPreferences{}
	if err := registerStartup(target, pref.persistenceBranding()); err != nil {
		t.Fatalf("register startup: %v", err)
	}

	data, err := os.ReadFile(runFile)
	if err != nil {
		t.Fatalf("read redirected run file: %v", err)
	}
	if string(data) != target {
		t.Fatalf("unexpected run file contents: %q", string(data))
	}

	if err := unregisterStartup(pref.persistenceBranding()); err != nil {
		t.Fatalf("unregister startup: %v", err)
	}

	if _, err := os.Stat(runFile); !os.IsNotExist(err) {
		t.Fatalf("run file still present after unregister: %v", err)
	}
}
