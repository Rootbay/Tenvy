//go:build !linux && !darwin && !windows

package agent

import (
	"fmt"
	"runtime"
)

func registerStartup(target string) error {
	return fmt.Errorf("startup registration not supported on %s", runtime.GOOS)
}

func unregisterStartup() error {
	return nil
}
