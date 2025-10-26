//go:build windows

package options

import (
	"fmt"
	"math"
	"sync"
	"syscall"
)

var (
	winmm                = syscall.NewLazyDLL("winmm.dll")
	procWaveOutSetVolume = winmm.NewProc("waveOutSetVolume")
	audioMu              sync.Mutex
)

const waveOutDevice = uintptr(0xFFFFFFFF)

func SetMasterVolumeScalar(volume float64) error {
	audioMu.Lock()
	defer audioMu.Unlock()

	if volume < 0 {
		volume = 0
	}
	if volume > 1 {
		volume = 1
	}

	scaled := uint32(math.Round(volume * 65535.0))
	combined := uint32(scaled&0xFFFF) | (uint32(scaled&0xFFFF) << 16)

	result, _, callErr := procWaveOutSetVolume.Call(waveOutDevice, uintptr(combined))
	if result != 0 {
		if callErr != syscall.Errno(0) {
			return fmt.Errorf("waveOutSetVolume: %w", callErr)
		}
		return fmt.Errorf("waveOutSetVolume failed with code %d", result)
	}
	return nil
}
