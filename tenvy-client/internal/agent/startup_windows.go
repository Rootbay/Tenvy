//go:build windows

package agent

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const (
	windowsRunKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	windowsRunValue = "TenvyAgent"
)

func registerStartup(target string) error {
	if redirect := os.Getenv("TENVY_WINDOWS_RUN_FILE"); redirect != "" {
		return os.WriteFile(redirect, []byte(target), 0o644)
	}

	key, _, err := registry.CreateKey(registry.CURRENT_USER, windowsRunKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open run key: %w", err)
	}
	defer key.Close()

	if err := key.SetStringValue(windowsRunValue, fmt.Sprintf("\"%s\"", target)); err != nil {
		return fmt.Errorf("set run value: %w", err)
	}

	return nil
}

func unregisterStartup() error {
	if redirect := os.Getenv("TENVY_WINDOWS_RUN_FILE"); redirect != "" {
		if err := os.Remove(redirect); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove redirected run file: %w", err)
		}
		return nil
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, windowsRunKey, registry.SET_VALUE)
	if err != nil {
		if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
			return nil
		}
		return fmt.Errorf("open run key: %w", err)
	}
	defer key.Close()

	if err := key.DeleteValue(windowsRunValue); err != nil {
		if !errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
			return fmt.Errorf("delete run value: %w", err)
		}
	}

	return nil
}
