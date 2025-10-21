//go:build linux

package remotedesktop

import (
	"errors"
	"fmt"
	"os"
	"sync"
)

var (
	vaapiProbeOnce sync.Once
	vaapiProbeErr  error
)

func probeVAAPI() error {
	vaapiProbeOnce.Do(func() {
		if _, err := os.Stat("/dev/dri/renderD128"); err != nil {
			vaapiProbeErr = ErrNativeEncoderUnavailable
			if !errors.Is(err, os.ErrNotExist) {
				vaapiProbeErr = fmt.Errorf("va-api device probe failed: %w", err)
			}
			return
		}
		vaapiProbeErr = ErrNativeEncoderUnavailable
	})
	return vaapiProbeErr
}

func platformNewNativeHEVCVideoEncoder() (clipVideoEncoder, error) {
	if err := probeVAAPI(); err != nil {
		return nil, err
	}
	return nil, ErrNativeEncoderUnavailable
}

func platformNewNativeAVCVideoEncoder() (clipVideoEncoder, error) {
	if err := probeVAAPI(); err != nil {
		return nil, err
	}
	return nil, ErrNativeEncoderUnavailable
}
