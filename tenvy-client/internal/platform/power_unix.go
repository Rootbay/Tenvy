//go:build !windows

package platform

import (
	"errors"
	"fmt"
	"os/exec"
	"os/user"
	"strings"
)

func requireElevation(action string) error {
	if !CurrentUserIsElevated() {
		return fmt.Errorf("%s requires elevated privileges", action)
	}
	return nil
}

func Shutdown() error {
	if err := requireElevation("shutdown"); err != nil {
		return err
	}
	return exec.Command("shutdown", "-h", "now").Run()
}

func Restart() error {
	if err := requireElevation("restart"); err != nil {
		return err
	}
	return exec.Command("shutdown", "-r", "now").Run()
}

func Sleep() error {
	if err := requireElevation("sleep"); err != nil {
		return err
	}

	attempts := [][]string{
		{"systemctl", "suspend"},
		{"pmset", "sleepnow"},
	}

	var errs []error
	for _, attempt := range attempts {
		cmd := exec.Command(attempt[0], attempt[1:]...)
		if err := cmd.Run(); err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				errs = append(errs, fmt.Errorf("%s not found", attempt[0]))
				continue
			}
			errs = append(errs, fmt.Errorf("%s failed: %w", attempt[0], err))
			continue
		}
		return nil
	}

	if len(errs) == 0 {
		return fmt.Errorf("no available suspend command")
	}

	return errors.Join(errs...)
}

func Logoff() error {
	if err := requireElevation("logoff"); err != nil {
		return err
	}

	username, err := currentUsername()
	if err != nil {
		return fmt.Errorf("resolve current user: %w", err)
	}

	attempts := [][]string{
		{"loginctl", "terminate-user", username},
		{"pkill", "-KILL", "-u", username},
	}

	var errs []error
	for _, attempt := range attempts {
		cmd := exec.Command(attempt[0], attempt[1:]...)
		if err := cmd.Run(); err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				errs = append(errs, fmt.Errorf("%s not found", attempt[0]))
				continue
			}
			errs = append(errs, fmt.Errorf("%s failed: %w", attempt[0], err))
			continue
		}
		return nil
	}

	if len(errs) == 0 {
		return fmt.Errorf("no available logoff command")
	}

	return errors.Join(errs...)
}

func currentUsername() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}

	username := strings.TrimSpace(u.Username)
	if username == "" {
		return "", fmt.Errorf("empty username returned")
	}

	return username, nil
}
