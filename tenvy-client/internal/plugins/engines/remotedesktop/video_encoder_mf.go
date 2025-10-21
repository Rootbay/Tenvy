//go:build windows

package remotedesktopengine

import (
	"fmt"
	"sync"
	"syscall"
)

var (
	mediaFoundationOnce sync.Once
	mediaFoundationErr  error
)

func ensureMediaFoundationRuntime() error {
	mediaFoundationOnce.Do(func() {
		mfplat := syscall.NewLazyDLL("mfplat.dll")
		if err := mfplat.Load(); err != nil {
			mediaFoundationErr = fmt.Errorf("media foundation runtime not available: %w", err)
			return
		}
		mediaFoundationErr = ErrNativeEncoderUnavailable
	})
	return mediaFoundationErr
}

func platformNewNativeHEVCVideoEncoder() (clipVideoEncoder, error) {
	if err := ensureMediaFoundationRuntime(); err != nil {
		return nil, err
	}
	return nil, ErrNativeEncoderUnavailable
}

func platformNewNativeAVCVideoEncoder() (clipVideoEncoder, error) {
	if err := ensureMediaFoundationRuntime(); err != nil {
		return nil, err
	}
	return nil, ErrNativeEncoderUnavailable
}
