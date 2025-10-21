//go:build darwin

package remotedesktop

import (
	"fmt"
	"sync"
	"syscall"
)

var (
	videoToolboxOnce sync.Once
	videoToolboxErr  error
)

func ensureVideoToolboxRuntime() error {
	videoToolboxOnce.Do(func() {
		framework := syscall.NewLazyDLL("/System/Library/Frameworks/VideoToolbox.framework/VideoToolbox")
		if err := framework.Load(); err != nil {
			videoToolboxErr = fmt.Errorf("videotoolbox framework not available: %w", err)
			return
		}
		videoToolboxErr = ErrNativeEncoderUnavailable
	})
	return videoToolboxErr
}

func platformNewNativeHEVCVideoEncoder() (clipVideoEncoder, error) {
	if err := ensureVideoToolboxRuntime(); err != nil {
		return nil, err
	}
	return nil, ErrNativeEncoderUnavailable
}

func platformNewNativeAVCVideoEncoder() (clipVideoEncoder, error) {
	if err := ensureVideoToolboxRuntime(); err != nil {
		return nil, err
	}
	return nil, ErrNativeEncoderUnavailable
}
