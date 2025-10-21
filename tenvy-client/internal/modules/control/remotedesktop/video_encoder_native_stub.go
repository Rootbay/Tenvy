//go:build !windows && !darwin && !linux

package remotedesktop

func platformNewNativeHEVCVideoEncoder() (clipVideoEncoder, error) {
	return nil, ErrNativeEncoderUnavailable
}

func platformNewNativeAVCVideoEncoder() (clipVideoEncoder, error) {
	return nil, ErrNativeEncoderUnavailable
}
