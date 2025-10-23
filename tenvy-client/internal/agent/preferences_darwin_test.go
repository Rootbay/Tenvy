//go:build darwin

package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegisterStartupPreferenceDarwin(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	target := filepath.Join(tmp, "Applications", "tenvy")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("prepare target dir: %v", err)
	}
	if err := os.WriteFile(target, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write target: %v", err)
	}

	pref := BuildPreferences{}
	if err := configureStartupPreference(pref, target); err != nil {
		t.Fatalf("configure startup: %v", err)
	}

	branding := pref.persistenceBranding()
	plistPath := filepath.Join(tmp, macLaunchAgentsDir, branding.LaunchAgentLabel+".plist")
	if _, err := os.Stat(plistPath); err != nil {
		t.Fatalf("stat plist: %v", err)
	}

	if err := unregisterStartup(branding); err != nil {
		t.Fatalf("unregister startup: %v", err)
	}

	if _, err := os.Stat(plistPath); !os.IsNotExist(err) {
		t.Fatalf("plist still present after unregister: %v", err)
	}
}
