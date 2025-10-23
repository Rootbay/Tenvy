//go:build linux

package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	linuxSystemdDir    = ".config/systemd/user"
	linuxServiceTarget = "default.target.wants"
)

func registerStartup(target string, branding PersistenceBranding) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	systemdDir := filepath.Join(homeDir, linuxSystemdDir)
	if err := os.MkdirAll(systemdDir, 0o755); err != nil {
		return fmt.Errorf("create systemd directory: %w", err)
	}

	serviceName := strings.TrimSpace(branding.ServiceName)
	if serviceName == "" {
		serviceName = "tenvy-agent.service"
	}

	description := strings.TrimSpace(branding.ServiceDescription)
	if description == "" {
		description = "Tenvy Agent"
	}

	servicePath := filepath.Join(systemdDir, serviceName)
	unit := fmt.Sprintf(`[Unit]
Description=%s
After=network.target

[Service]
Type=simple
ExecStart="%s"
Restart=on-failure

[Install]
WantedBy=default.target
`, description, target)

	if err := os.WriteFile(servicePath, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("write systemd unit: %w", err)
	}

	wantsDir := filepath.Join(systemdDir, linuxServiceTarget)
	if err := os.MkdirAll(wantsDir, 0o755); err != nil {
		return fmt.Errorf("create wants directory: %w", err)
	}

	linkPath := filepath.Join(wantsDir, serviceName)
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

	cronFile := strings.TrimSpace(branding.CronFilename)
	if cronFile == "" {
		cronFile = "tenvy-agent.cron"
	}

	cronPath := filepath.Join(cronDir, cronFile)
	cronEntry := fmt.Sprintf("@reboot %s\n", target)
	if err := os.WriteFile(cronPath, []byte(cronEntry), 0o644); err != nil {
		return fmt.Errorf("write cron entry: %w", err)
	}

	return nil
}

func unregisterStartup(branding PersistenceBranding) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	systemdDir := filepath.Join(homeDir, linuxSystemdDir)
	serviceName := strings.TrimSpace(branding.ServiceName)
	if serviceName == "" {
		serviceName = "tenvy-agent.service"
	}

	servicePath := filepath.Join(systemdDir, serviceName)
	wantsDir := filepath.Join(systemdDir, linuxServiceTarget)
	linkPath := filepath.Join(wantsDir, serviceName)

	if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove wants symlink: %w", err)
	}

	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove systemd unit: %w", err)
	}

	cronFile := strings.TrimSpace(branding.CronFilename)
	if cronFile == "" {
		cronFile = "tenvy-agent.cron"
	}

	cronPath := filepath.Join(homeDir, ".config", "cron", cronFile)
	if err := os.Remove(cronPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove cron entry: %w", err)
	}

	return nil
}
