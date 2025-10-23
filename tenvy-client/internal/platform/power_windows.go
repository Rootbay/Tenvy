//go:build windows

package platform

import (
	"fmt"
	"os/exec"
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
	return exec.Command("shutdown", "/s", "/t", "0").Run()
}

func Restart() error {
	if err := requireElevation("restart"); err != nil {
		return err
	}
	return exec.Command("shutdown", "/r", "/t", "0").Run()
}

func Sleep() error {
	if err := requireElevation("sleep"); err != nil {
		return err
	}
	return exec.Command("rundll32.exe", "powrprof.dll,SetSuspendState", "0,1,0").Run()
}

func Logoff() error {
	if err := requireElevation("logoff"); err != nil {
		return err
	}
	return exec.Command("shutdown", "/l").Run()
}
