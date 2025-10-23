//go:build windows

package agent

import (
	"time"

	"golang.org/x/sys/windows"
)

func systemUptime() (time.Duration, error) {
	return windows.DurationSinceBoot(), nil
}
