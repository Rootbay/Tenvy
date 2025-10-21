//go:build windows

package agent

import (
	"syscall"
	"time"
)

func systemUptime() (time.Duration, error) {
	ticks := syscall.GetTickCount64()
	return time.Duration(ticks) * time.Millisecond, nil
}
