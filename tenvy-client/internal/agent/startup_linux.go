//go:build linux

package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	linuxSystemdDir    = ".config/systemd/user"
	linuxServiceName   = "tenvy-agent.service"
	linuxServiceTarget = "default.target.wants"
)

func registerStartup(target string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	systemdDir := filepath.Join(homeDir, linuxSystemdDir)
	if err := os.MkdirAll(systemdDir, 0o755); err != nil {
		return fmt.Errorf("create systemd directory: %w", err)
	}

	servicePath := filepath.Join(systemdDir, linuxServiceName)
	unit := fmt.Sprintf(`[Unit]
Description=Tenvy Agent
After=network.target

[Service]
Type=simple
ExecStart="%s"
Restart=on-failure

[Install]
WantedBy=default.target
`, target)

	if err := os.WriteFile(servicePath, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("write systemd unit: %w", err)
	}

	wantsDir := filepath.Join(systemdDir, linuxServiceTarget)
	if err := os.MkdirAll(wantsDir, 0o755); err != nil {
		return fmt.Errorf("create wants directory: %w", err)
	}

	linkPath := filepath.Join(wantsDir, linuxServiceName)
	if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("replace wants symlink: %w", err)
	}

	if err := os.Symlink(servicePath, linkPath); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("link systemd unit: %w", err)
		}
	}

	cronDir := filepath.Join(homeDir, ".config", "cron")
	if err := os.MkdirAll(cronDir, 0o755); err != nil {
		return fmt.Errorf("create cron directory: %w", err)
	}

	cronPath := filepath.Join(cronDir, "tenvy-agent.cron")
	cronEntry := fmt.Sprintf("@reboot %s\n", target)
	if err := os.WriteFile(cronPath, []byte(cronEntry), 0o644); err != nil {
		return fmt.Errorf("write cron entry: %w", err)
	}

	return nil
}

func unregisterStartup() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	systemdDir := filepath.Join(homeDir, linuxSystemdDir)
	servicePath := filepath.Join(systemdDir, linuxServiceName)
	wantsDir := filepath.Join(systemdDir, linuxServiceTarget)
	linkPath := filepath.Join(wantsDir, linuxServiceName)

	if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove wants symlink: %w", err)
	}

	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove systemd unit: %w", err)
	}

	cronPath := filepath.Join(homeDir, ".config", "cron", "tenvy-agent.cron")
	if err := os.Remove(cronPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove cron entry: %w", err)
	}

	return nil
}
