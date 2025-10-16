package remotedesktop

import "strings"

func defaultRemoteDesktopSettings() RemoteDesktopSettings {
	return RemoteDesktopSettings{
		Quality:  RemoteQualityAuto,
		Monitor:  0,
		Mouse:    true,
		Keyboard: true,
		Mode:     RemoteStreamModeVideo,
		Encoder:  RemoteEncoderAuto,
	}
}

func applySettingsPatch(settings *RemoteDesktopSettings, patch *RemoteDesktopSettingsPatch) {
	if settings == nil || patch == nil {
		return
	}

	if patch.Quality != nil {
		settings.Quality = normalizeQuality(*patch.Quality)
	}
	if patch.Monitor != nil {
		settings.Monitor = *patch.Monitor
	}
	if patch.Mouse != nil {
		settings.Mouse = *patch.Mouse
	}
	if patch.Keyboard != nil {
		settings.Keyboard = *patch.Keyboard
	}
	if patch.Mode != nil {
		settings.Mode = normalizeStreamMode(*patch.Mode)
	}
	if patch.Encoder != nil {
		settings.Encoder = normalizeEncoder(*patch.Encoder)
	}
}

func normalizeQuality(value RemoteDesktopQuality) RemoteDesktopQuality {
	switch strings.ToLower(string(value)) {
	case string(RemoteQualityHigh):
		return RemoteQualityHigh
	case string(RemoteQualityMedium):
		return RemoteQualityMedium
	case string(RemoteQualityLow):
		return RemoteQualityLow
	case string(RemoteQualityAuto):
		return RemoteQualityAuto
	default:
		return RemoteQualityAuto
	}
}

func normalizeStreamMode(value RemoteDesktopStreamMode) RemoteDesktopStreamMode {
	switch strings.ToLower(string(value)) {
	case string(RemoteStreamModeImages):
		return RemoteStreamModeImages
	case string(RemoteStreamModeVideo):
		return RemoteStreamModeVideo
	default:
		return RemoteStreamModeVideo
	}
}

func normalizeEncoder(value RemoteDesktopEncoder) RemoteDesktopEncoder {
	switch strings.ToLower(string(value)) {
	case string(RemoteEncoderHEVC):
		return RemoteEncoderHEVC
	case string(RemoteEncoderAVC):
		return RemoteEncoderAVC
	case string(RemoteEncoderJPEG):
		return RemoteEncoderJPEG
	case string(RemoteEncoderAuto):
		fallthrough
	default:
		return RemoteEncoderAuto
	}
}
