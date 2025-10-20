package remotedesktop

import "strings"

func defaultRemoteDesktopSettings() RemoteDesktopSettings {
	return RemoteDesktopSettings{
		Quality:           RemoteQualityAuto,
		Monitor:           0,
		Mouse:             true,
		Keyboard:          true,
		Mode:              RemoteStreamModeVideo,
		Encoder:           RemoteEncoderAuto,
		Transport:         RemoteTransportWebRTC,
		Hardware:          RemoteHardwareAuto,
		TargetBitrateKbps: 0,
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
	if patch.Transport != nil {
		settings.Transport = normalizeTransport(*patch.Transport)
	}
	if patch.Hardware != nil {
		settings.Hardware = normalizeHardware(*patch.Hardware)
	}
	if patch.TargetBitrateKbps != nil {
		target := *patch.TargetBitrateKbps
		if target <= 0 {
			settings.TargetBitrateKbps = 0
		} else {
			settings.TargetBitrateKbps = target
		}
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

func normalizeTransport(value RemoteDesktopTransport) RemoteDesktopTransport {
	switch strings.ToLower(string(value)) {
	case string(RemoteTransportHTTP):
		return RemoteTransportHTTP
	case string(RemoteTransportWebRTC):
		fallthrough
	default:
		return RemoteTransportWebRTC
	}
}

func normalizeHardware(value RemoteDesktopHardwarePreference) RemoteDesktopHardwarePreference {
	switch strings.ToLower(string(value)) {
	case string(RemoteHardwarePrefer):
		return RemoteHardwarePrefer
	case string(RemoteHardwareAvoid):
		return RemoteHardwareAvoid
	case string(RemoteHardwareAuto):
		fallthrough
	default:
		return RemoteHardwareAuto
	}
}
