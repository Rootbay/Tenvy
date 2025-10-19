//go:build linux

package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestConfigureStartupPreferenceLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux specific test")
	}

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	target := filepath.Join(tmp, "bin", "tenvy-agent")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("prepare target dir: %v", err)
	}
	if err := os.WriteFile(target, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write target binary: %v", err)
	}

	if err := configureStartupPreference(target); err != nil {
		t.Fatalf("configure startup: %v", err)
	}

	entryPath := filepath.Join(tmp, ".config", "tenvy", "startup-target.txt")
	data, err := os.ReadFile(entryPath)
	if err != nil {
		t.Fatalf("read startup entry: %v", err)
	}
	if string(data) != target+"\n" {
		t.Fatalf("unexpected startup entry contents: %q", string(data))
	}

	systemdDir := filepath.Join(tmp, ".config", "systemd", "user")
	servicePath := filepath.Join(systemdDir, linuxServiceName)
	unit, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("read systemd unit: %v", err)
	}
	if !strings.Contains(string(unit), target) {
		t.Fatalf("systemd unit missing target: %s", string(unit))
	}

	wantsLink := filepath.Join(systemdDir, linuxServiceTarget, linuxServiceName)
	info, err := os.Lstat(wantsLink)
	if err != nil {
		t.Fatalf("lstat wants link: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink at %s", wantsLink)
	}
	linkDest, err := os.Readlink(wantsLink)
	if err != nil {
		t.Fatalf("read symlink: %v", err)
	}
	if linkDest != servicePath {
		t.Fatalf("unexpected symlink destination: %s", linkDest)
	}

	cronPath := filepath.Join(tmp, ".config", "cron", "tenvy-agent.cron")
	cronData, err := os.ReadFile(cronPath)
	if err != nil {
		t.Fatalf("read cron entry: %v", err)
	}
	if string(cronData) != "@reboot "+target+"\n" {
		t.Fatalf("unexpected cron contents: %q", string(cronData))
	}

	if err := unregisterStartup(); err != nil {
		t.Fatalf("unregister startup: %v", err)
	}

	if _, err := os.Stat(servicePath); !os.IsNotExist(err) {
		t.Fatalf("systemd unit still present after unregister: %v", err)
	}
	if _, err := os.Lstat(wantsLink); !os.IsNotExist(err) {
		t.Fatalf("wants link still present after unregister: %v", err)
	}
	if _, err := os.Stat(cronPath); !os.IsNotExist(err) {
		t.Fatalf("cron entry still present after unregister: %v", err)
	}
}
