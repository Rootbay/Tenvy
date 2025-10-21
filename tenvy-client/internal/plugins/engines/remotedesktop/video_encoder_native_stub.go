//go:build !windows && !darwin && !linux

package remotedesktopengine

func platformNewNativeHEVCVideoEncoder() (clipVideoEncoder, error) {
	return nil, ErrNativeEncoderUnavailable
}

func platformNewNativeAVCVideoEncoder() (clipVideoEncoder, error) {
	return nil, ErrNativeEncoderUnavailable
}
