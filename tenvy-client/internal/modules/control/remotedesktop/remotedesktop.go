package remotedesktop

import (
	engine "github.com/rootbay/tenvy-client/internal/plugins/engines/remotedesktop"
)

type (
	Logger                                  = engine.Logger
	HTTPDoer                                = engine.HTTPDoer
	Config                                  = engine.Config
	RemoteDesktopQuality                    = engine.RemoteDesktopQuality
	RemoteDesktopStreamMode                 = engine.RemoteDesktopStreamMode
	RemoteDesktopEncoder                    = engine.RemoteDesktopEncoder
	RemoteDesktopTransport                  = engine.RemoteDesktopTransport
	RemoteDesktopHardwarePreference         = engine.RemoteDesktopHardwarePreference
	RemoteDesktopSettings                   = engine.RemoteDesktopSettings
	RemoteDesktopSettingsPatch              = engine.RemoteDesktopSettingsPatch
	RemoteDesktopCommandPayload             = engine.RemoteDesktopCommandPayload
	RemoteDesktopInputEvent                 = engine.RemoteDesktopInputEvent
	RemoteDesktopInputType                  = engine.RemoteDesktopInputType
	RemoteDesktopMouseButton                = engine.RemoteDesktopMouseButton
	RemoteDesktopFrameMetrics               = engine.RemoteDesktopFrameMetrics
	RemoteDesktopTransportDiagnostics       = engine.RemoteDesktopTransportDiagnostics
	RemoteDesktopMonitorInfo                = engine.RemoteDesktopMonitorInfo
	RemoteDesktopDeltaRect                  = engine.RemoteDesktopDeltaRect
	RemoteDesktopMediaSample                = engine.RemoteDesktopMediaSample
	RemoteDesktopFramePacket                = engine.RemoteDesktopFramePacket
	RemoteDesktopTransportCapability        = engine.RemoteDesktopTransportCapability
	QUICInputConfig                         = engine.QUICInputConfig
	RemoteDesktopInputQuicConfig            = engine.RemoteDesktopInputQuicConfig
	RemoteDesktopInputNegotiation           = engine.RemoteDesktopInputNegotiation
	RemoteDesktopSessionNegotiationRequest  = engine.RemoteDesktopSessionNegotiationRequest
	RemoteDesktopSessionNegotiationResponse = engine.RemoteDesktopSessionNegotiationResponse
	RemoteDesktopWebRTCOffer                = engine.RemoteDesktopWebRTCOffer
	RemoteDesktopWebRTCAnswer               = engine.RemoteDesktopWebRTCAnswer
	RemoteDesktopWebRTCICEServer            = engine.RemoteDesktopWebRTCICEServer
	RemoteDesktopVideoClip                  = engine.RemoteDesktopVideoClip
	RemoteDesktopClipFrame                  = engine.RemoteDesktopClipFrame
	RemoteDesktopSession                    = engine.RemoteDesktopSession
	Engine                                  = engine.Engine
	RemoteDesktopStreamer                   = engine.RemoteDesktopStreamer
	ClipEncoderEvent                        = engine.ClipEncoderEvent
	ClipEncoderProfiler                     = engine.ClipEncoderProfiler
)

const (
	RemoteQualityAuto   = engine.RemoteQualityAuto
	RemoteQualityHigh   = engine.RemoteQualityHigh
	RemoteQualityMedium = engine.RemoteQualityMedium
	RemoteQualityLow    = engine.RemoteQualityLow

	RemoteStreamModeImages = engine.RemoteStreamModeImages
	RemoteStreamModeVideo  = engine.RemoteStreamModeVideo

	RemoteEncoderAuto = engine.RemoteEncoderAuto
	RemoteEncoderHEVC = engine.RemoteEncoderHEVC
	RemoteEncoderAVC  = engine.RemoteEncoderAVC
	RemoteEncoderJPEG = engine.RemoteEncoderJPEG

	RemoteTransportHTTP   = engine.RemoteTransportHTTP
	RemoteTransportWebRTC = engine.RemoteTransportWebRTC

	RemoteHardwareAuto   = engine.RemoteHardwareAuto
	RemoteHardwarePrefer = engine.RemoteHardwarePrefer
	RemoteHardwareAvoid  = engine.RemoteHardwareAvoid

	RemoteInputMouseMove   = engine.RemoteInputMouseMove
	RemoteInputMouseButton = engine.RemoteInputMouseButton
	RemoteInputMouseScroll = engine.RemoteInputMouseScroll
	RemoteInputKey         = engine.RemoteInputKey

	RemoteMouseButtonLeft   = engine.RemoteMouseButtonLeft
	RemoteMouseButtonMiddle = engine.RemoteMouseButtonMiddle
	RemoteMouseButtonRight  = engine.RemoteMouseButtonRight
)

var (
	SetClipEncoderProfiler   = engine.SetClipEncoderProfiler
	NewRemoteDesktopStreamer = engine.NewRemoteDesktopStreamer
	DecodeCommandPayload     = engine.DecodeCommandPayload
)
