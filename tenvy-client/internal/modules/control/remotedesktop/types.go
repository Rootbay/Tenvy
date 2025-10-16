package remotedesktop

import (
	"context"
	"image"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

type (
	Command       = protocol.Command
	CommandResult = protocol.CommandResult
)

type Logger interface {
	Printf(format string, args ...interface{})
}

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Config struct {
	AgentID        string
	BaseURL        string
	AuthKey        string
	Client         HTTPDoer
	Logger         Logger
	UserAgent      string
	RequestTimeout time.Duration
	authHeader     string
}

type RemoteDesktopQuality string

type RemoteDesktopStreamMode string

type RemoteDesktopEncoder string

const (
	RemoteQualityAuto   RemoteDesktopQuality = "auto"
	RemoteQualityHigh   RemoteDesktopQuality = "high"
	RemoteQualityMedium RemoteDesktopQuality = "medium"
	RemoteQualityLow    RemoteDesktopQuality = "low"

	RemoteStreamModeImages RemoteDesktopStreamMode = "images"
	RemoteStreamModeVideo  RemoteDesktopStreamMode = "video"
)

const (
	RemoteEncoderAuto RemoteDesktopEncoder = "auto"
	RemoteEncoderHEVC RemoteDesktopEncoder = "hevc"
	RemoteEncoderAVC  RemoteDesktopEncoder = "avc"
	RemoteEncoderJPEG RemoteDesktopEncoder = "jpeg"
)

type RemoteDesktopInputType string

const (
	RemoteInputMouseMove   RemoteDesktopInputType = "mouse-move"
	RemoteInputMouseButton RemoteDesktopInputType = "mouse-button"
	RemoteInputMouseScroll RemoteDesktopInputType = "mouse-scroll"
	RemoteInputKey         RemoteDesktopInputType = "key"
)

type RemoteDesktopMouseButton string

const (
	RemoteMouseButtonLeft   RemoteDesktopMouseButton = "left"
	RemoteMouseButtonMiddle RemoteDesktopMouseButton = "middle"
	RemoteMouseButtonRight  RemoteDesktopMouseButton = "right"
)

type RemoteDesktopSettings struct {
	Quality  RemoteDesktopQuality    `json:"quality"`
	Monitor  int                     `json:"monitor"`
	Mouse    bool                    `json:"mouse"`
	Keyboard bool                    `json:"keyboard"`
	Mode     RemoteDesktopStreamMode `json:"mode"`
	Encoder  RemoteDesktopEncoder    `json:"encoder,omitempty"`
}

type RemoteDesktopSettingsPatch struct {
	Quality  *RemoteDesktopQuality    `json:"quality,omitempty"`
	Monitor  *int                     `json:"monitor,omitempty"`
	Mouse    *bool                    `json:"mouse,omitempty"`
	Keyboard *bool                    `json:"keyboard,omitempty"`
	Mode     *RemoteDesktopStreamMode `json:"mode,omitempty"`
	Encoder  *RemoteDesktopEncoder    `json:"encoder,omitempty"`
}

type RemoteDesktopCommandPayload struct {
	Action    string                      `json:"action"`
	SessionID string                      `json:"sessionId,omitempty"`
	Settings  *RemoteDesktopSettingsPatch `json:"settings,omitempty"`
	Events    []RemoteDesktopInputEvent   `json:"events,omitempty"`
}

type RemoteDesktopInputEvent struct {
	Type       RemoteDesktopInputType   `json:"type"`
	X          float64                  `json:"x,omitempty"`
	Y          float64                  `json:"y,omitempty"`
	Normalized bool                     `json:"normalized,omitempty"`
	Monitor    *int                     `json:"monitor,omitempty"`
	Button     RemoteDesktopMouseButton `json:"button,omitempty"`
	Pressed    bool                     `json:"pressed,omitempty"`
	DeltaX     float64                  `json:"deltaX,omitempty"`
	DeltaY     float64                  `json:"deltaY,omitempty"`
	DeltaMode  int                      `json:"deltaMode,omitempty"`
	Key        string                   `json:"key,omitempty"`
	Code       string                   `json:"code,omitempty"`
	KeyCode    int                      `json:"keyCode,omitempty"`
	Repeat     bool                     `json:"repeat,omitempty"`
	AltKey     bool                     `json:"altKey,omitempty"`
	CtrlKey    bool                     `json:"ctrlKey,omitempty"`
	ShiftKey   bool                     `json:"shiftKey,omitempty"`
	MetaKey    bool                     `json:"metaKey,omitempty"`
}

type RemoteDesktopFrameMetrics struct {
	FPS                 float64 `json:"fps,omitempty"`
	BandwidthKbps       float64 `json:"bandwidthKbps,omitempty"`
	CPUPercent          float64 `json:"cpuPercent,omitempty"`
	GPUPercent          float64 `json:"gpuPercent,omitempty"`
	ClipQuality         int     `json:"clipQuality,omitempty"`
	CaptureLatencyMs    float64 `json:"captureLatencyMs,omitempty"`
	EncodeLatencyMs     float64 `json:"encodeLatencyMs,omitempty"`
	ProcessingLatencyMs float64 `json:"processingLatencyMs,omitempty"`
	FrameJitterMs       float64 `json:"frameJitterMs,omitempty"`
	TargetBitrateKbps   float64 `json:"targetBitrateKbps,omitempty"`
	LadderLevel         int     `json:"ladderLevel,omitempty"`
	FrameLossPercent    float64 `json:"frameLossPercent,omitempty"`
}

type RemoteDesktopMonitorInfo struct {
	ID     int    `json:"id"`
	Label  string `json:"label"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type RemoteDesktopDeltaRect struct {
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Encoding string `json:"encoding"`
	Data     string `json:"data"`
}

type RemoteDesktopFramePacket struct {
	SessionID string                     `json:"sessionId"`
	Sequence  uint64                     `json:"sequence"`
	Timestamp string                     `json:"timestamp"`
	Width     int                        `json:"width"`
	Height    int                        `json:"height"`
	KeyFrame  bool                       `json:"keyFrame"`
	Encoding  string                     `json:"encoding"`
	Image     string                     `json:"image,omitempty"`
	Deltas    []RemoteDesktopDeltaRect   `json:"deltas,omitempty"`
	Clip      *RemoteDesktopVideoClip    `json:"clip,omitempty"`
	Encoder   RemoteDesktopEncoder       `json:"encoder,omitempty"`
	Monitors  []RemoteDesktopMonitorInfo `json:"monitors,omitempty"`
	Metrics   *RemoteDesktopFrameMetrics `json:"metrics,omitempty"`
}

type RemoteDesktopVideoClip struct {
	DurationMs int                      `json:"durationMs"`
	Frames     []RemoteDesktopClipFrame `json:"frames"`
}

type RemoteDesktopClipFrame struct {
	OffsetMs int    `json:"offsetMs"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Encoding string `json:"encoding"`
	Data     string `json:"data"`
}

type RemoteDesktopSession struct {
	ID                 string
	Settings           RemoteDesktopSettings
	ActiveEncoder      RemoteDesktopEncoder
	Width              int
	Height             int
	TileSize           int
	ClipQuality        int
	BaseClipQuality    int
	FrameInterval      time.Duration
	Sequence           uint64
	LastFrame          []byte
	ForceKeyFrame      bool
	BaseWidth          int
	BaseHeight         int
	NativeWidth        int
	NativeHeight       int
	AdaptiveScale      float64
	MinScale           float64
	MaxScale           float64
	BaseInterval       time.Duration
	MinInterval        time.Duration
	MaxInterval        time.Duration
	BaseTile           int
	MinTile            int
	MaxTile            int
	MinClipQuality     int
	MaxClipQuality     int
	LastAdaptation     time.Time
	qualityLadder      []remoteQualityProfile
	ladderIndex        int
	TargetBitrateKbps  int
	bandwidthEMA       float64
	fpsEMA             float64
	processingEMA      float64
	frameDropEMA       float64
	monitors           []remoteMonitor
	monitorInfos       []RemoteDesktopMonitorInfo
	monitorsDirty      bool
	lastMonitorRefresh time.Time
	ctx                context.Context
	cancel             context.CancelCauseFunc
	wg                 sync.WaitGroup
}

type remoteMonitor struct {
	info   RemoteDesktopMonitorInfo
	bounds image.Rectangle
}

type RemoteDesktopStreamer struct {
	controller *remoteDesktopSessionController
}

type remoteDesktopSessionController struct {
	cfg           atomic.Value // stores Config
	mu            sync.Mutex
	session       *RemoteDesktopSession
	endpointCache atomic.Value
}
