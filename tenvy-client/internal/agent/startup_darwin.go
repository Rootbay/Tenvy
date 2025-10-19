//go:build darwin

package agent

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	macLaunchAgentsDir = "Library/LaunchAgents"
	macPlistName       = "com.tenvy.agent.plist"
)

func registerStartup(target string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	launchDir := filepath.Join(homeDir, macLaunchAgentsDir)
	if err := os.MkdirAll(launchDir, 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents directory: %w", err)
	}

	plistPath := filepath.Join(launchDir, macPlistName)
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.tenvy.agent</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
</dict>
</plist>
`, target)

	if err := os.WriteFile(plistPath, []byte(plist), 0o644); err != nil {
		return fmt.Errorf("write LaunchAgent plist: %w", err)
	}

	return nil
}

func unregisterStartup() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	plistPath := filepath.Join(homeDir, macLaunchAgentsDir, macPlistName)
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove LaunchAgent plist: %w", err)
	}

	return nil
}
