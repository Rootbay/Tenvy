package remotedesktopengine

import (
	"context"
	"crypto/x509"
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

type frameTransport interface {
	Send(ctx context.Context, frame RemoteDesktopFramePacket) error
	Close() error
	Ready() bool
}

type Config struct {
	AgentID          string
	BaseURL          string
	AuthKey          string
	PluginVersion    string
	Client           HTTPDoer
	Logger           Logger
	UserAgent        string
	RequestTimeout   time.Duration
	WebRTCICEServers []RemoteDesktopWebRTCICEServer
	QUICInput        QUICInputConfig
	authHeader       string
	quicInput        sanitizedQUICInput
}

type RemoteDesktopQuality string

type RemoteDesktopStreamMode string

type RemoteDesktopEncoder string

type RemoteDesktopTransport string

type RemoteDesktopHardwarePreference string

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

const (
	RemoteTransportHTTP   RemoteDesktopTransport = "http"
	RemoteTransportWebRTC RemoteDesktopTransport = "webrtc"
)

const spkiHashLength = 32

const (
	RemoteHardwareAuto   RemoteDesktopHardwarePreference = "auto"
	RemoteHardwarePrefer RemoteDesktopHardwarePreference = "prefer"
	RemoteHardwareAvoid  RemoteDesktopHardwarePreference = "avoid"
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
	Quality           RemoteDesktopQuality            `json:"quality"`
	Monitor           int                             `json:"monitor"`
	Mouse             bool                            `json:"mouse"`
	Keyboard          bool                            `json:"keyboard"`
	Mode              RemoteDesktopStreamMode         `json:"mode"`
	Encoder           RemoteDesktopEncoder            `json:"encoder,omitempty"`
	Transport         RemoteDesktopTransport          `json:"transport,omitempty"`
	Hardware          RemoteDesktopHardwarePreference `json:"hardware,omitempty"`
	TargetBitrateKbps int                             `json:"targetBitrateKbps,omitempty"`
}

type RemoteDesktopSettingsPatch struct {
	Quality           *RemoteDesktopQuality            `json:"quality,omitempty"`
	Monitor           *int                             `json:"monitor,omitempty"`
	Mouse             *bool                            `json:"mouse,omitempty"`
	Keyboard          *bool                            `json:"keyboard,omitempty"`
	Mode              *RemoteDesktopStreamMode         `json:"mode,omitempty"`
	Encoder           *RemoteDesktopEncoder            `json:"encoder,omitempty"`
	Transport         *RemoteDesktopTransport          `json:"transport,omitempty"`
	Hardware          *RemoteDesktopHardwarePreference `json:"hardware,omitempty"`
	TargetBitrateKbps *int                             `json:"targetBitrateKbps,omitempty"`
}

type RemoteDesktopCommandPayload struct {
	Action    string                      `json:"action"`
	SessionID string                      `json:"sessionId,omitempty"`
	Settings  *RemoteDesktopSettingsPatch `json:"settings,omitempty"`
	Events    []RemoteDesktopInputEvent   `json:"events,omitempty"`
}

type RemoteDesktopInputEvent struct {
	Type       RemoteDesktopInputType   `json:"type"`
	CapturedAt int64                    `json:"capturedAt"`
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
	CaptureLatencyMs    float64 `json:"captureLatencyMs,omitempty"`
	EncodeLatencyMs     float64 `json:"encodeLatencyMs,omitempty"`
	ProcessingLatencyMs float64 `json:"processingLatencyMs,omitempty"`
	FrameJitterMs       float64 `json:"frameJitterMs,omitempty"`
	TargetBitrateKbps   float64 `json:"targetBitrateKbps,omitempty"`
	LadderLevel         int     `json:"ladderLevel,omitempty"`
	FrameLossPercent    float64 `json:"frameLossPercent,omitempty"`
}

type RemoteDesktopTransportDiagnostics struct {
	Transport             RemoteDesktopTransport `json:"transport"`
	Codec                 RemoteDesktopEncoder   `json:"codec,omitempty"`
	BandwidthEstimateKbps float64                `json:"bandwidthEstimateKbps,omitempty"`
	AvailableBitrateKbps  float64                `json:"availableBitrateKbps,omitempty"`
	CurrentBitrateKbps    float64                `json:"currentBitrateKbps,omitempty"`
	RTTMs                 float64                `json:"rttMs,omitempty"`
	JitterMs              float64                `json:"jitterMs,omitempty"`
	PacketsLost           float64                `json:"packetsLost,omitempty"`
	FramesDropped         float64                `json:"framesDropped,omitempty"`
	LastUpdatedAt         string                 `json:"lastUpdatedAt,omitempty"`
	HardwareFallback      bool                   `json:"hardwareFallback,omitempty"`
	HardwareEncoder       string                 `json:"hardwareEncoder,omitempty"`
}

type RemoteDesktopMonitorInfo struct {
	ID     int    `json:"id"`
	Label  string `json:"label"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type RemoteDesktopDeltaRect struct {
	X        int    `json:"x" msgpack:"x"`
	Y        int    `json:"y" msgpack:"y"`
	Width    int    `json:"width" msgpack:"width"`
	Height   int    `json:"height" msgpack:"height"`
	Encoding string `json:"encoding" msgpack:"encoding"`
	Data     []byte `json:"data" msgpack:"data"`
}

type RemoteDesktopMediaSample struct {
	Kind      string `json:"kind" msgpack:"kind"`
	Codec     string `json:"codec" msgpack:"codec"`
	Timestamp int64  `json:"timestamp" msgpack:"timestamp"`
	KeyFrame  bool   `json:"keyFrame,omitempty" msgpack:"keyFrame,omitempty"`
	Format    string `json:"format,omitempty" msgpack:"format,omitempty"`
	Data      []byte `json:"data" msgpack:"data"`
}

type RemoteDesktopFramePacket struct {
	SessionID       string                     `json:"sessionId" msgpack:"sessionId"`
	Sequence        uint64                     `json:"sequence" msgpack:"sequence"`
	Timestamp       string                     `json:"timestamp" msgpack:"timestamp"`
	Width           int                        `json:"width" msgpack:"width"`
	Height          int                        `json:"height" msgpack:"height"`
	KeyFrame        bool                       `json:"keyFrame" msgpack:"keyFrame"`
	Encoding        string                     `json:"encoding" msgpack:"encoding"`
	Transport       RemoteDesktopTransport     `json:"transport,omitempty" msgpack:"transport,omitempty"`
	Image           []byte                     `json:"image,omitempty" msgpack:"image,omitempty"`
	Deltas          []RemoteDesktopDeltaRect   `json:"deltas,omitempty" msgpack:"deltas,omitempty"`
	Clip            *RemoteDesktopVideoClip    `json:"clip,omitempty" msgpack:"clip,omitempty"`
	Encoder         RemoteDesktopEncoder       `json:"encoder,omitempty" msgpack:"encoder,omitempty"`
	EncoderHardware string                     `json:"encoderHardware,omitempty" msgpack:"encoderHardware,omitempty"`
	IntraRefresh    bool                       `json:"intraRefresh,omitempty" msgpack:"intraRefresh,omitempty"`
	Monitors        []RemoteDesktopMonitorInfo `json:"monitors,omitempty" msgpack:"monitors,omitempty"`
	Metrics         *RemoteDesktopFrameMetrics `json:"metrics,omitempty" msgpack:"metrics,omitempty"`
	Media           []RemoteDesktopMediaSample `json:"media,omitempty" msgpack:"media,omitempty"`
}

type RemoteDesktopTransportCapability struct {
	Transport RemoteDesktopTransport `json:"transport"`
	Codecs    []RemoteDesktopEncoder `json:"codecs"`
	Features  map[string]bool        `json:"features,omitempty"`
}

type QUICInputConfig struct {
	URL                string        `json:"url,omitempty"`
	Token              string        `json:"token,omitempty"`
	ALPN               string        `json:"alpn,omitempty"`
	Disabled           bool          `json:"disabled,omitempty"`
	ConnectTimeout     time.Duration `json:"connectTimeout,omitempty"`
	RetryInterval      time.Duration `json:"retryInterval,omitempty"`
	InsecureSkipVerify bool          `json:"insecureSkipVerify,omitempty"`
	RootCAFiles        []string      `json:"rootCaFiles,omitempty"`
	RootCAPEMs         []string      `json:"rootCaPems,omitempty"`
	PinnedSPKIHashes   []string      `json:"pinnedSpkiHashes,omitempty"`
}

type sanitizedQUICInput struct {
	enabled        bool
	address        string
	serverName     string
	alpn           string
	token          string
	connectTimeout time.Duration
	retryInterval  time.Duration
	rootCAs        *x509.CertPool
	spkiPins       [][]byte
}

type RemoteDesktopInputQuicConfig struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port,omitempty"`
	ALPN    string `json:"alpn,omitempty"`
}

type RemoteDesktopInputNegotiation struct {
	QUIC *RemoteDesktopInputQuicConfig `json:"quic,omitempty"`
}

type RemoteDesktopSessionNegotiationRequest struct {
	SessionID     string                             `json:"sessionId"`
	Transports    []RemoteDesktopTransportCapability `json:"transports"`
	Codecs        []RemoteDesktopEncoder             `json:"codecs,omitempty"`
	IntraRefresh  bool                               `json:"intraRefresh,omitempty"`
	PluginVersion string                             `json:"pluginVersion,omitempty"`
	WebRTC        *RemoteDesktopWebRTCOffer          `json:"webrtc,omitempty"`
}

type RemoteDesktopSessionNegotiationResponse struct {
	Accepted              bool                           `json:"accepted"`
	Transport             RemoteDesktopTransport         `json:"transport,omitempty"`
	Codec                 RemoteDesktopEncoder           `json:"codec,omitempty"`
	IntraRefresh          bool                           `json:"intraRefresh,omitempty"`
	Features              map[string]bool                `json:"features,omitempty"`
	Reason                string                         `json:"reason,omitempty"`
	RequiredPluginVersion string                         `json:"requiredPluginVersion,omitempty"`
	WebRTC                *RemoteDesktopWebRTCAnswer     `json:"webrtc,omitempty"`
	Input                 *RemoteDesktopInputNegotiation `json:"input,omitempty"`
}

type RemoteDesktopWebRTCOffer struct {
	Offer       string                         `json:"offer"`
	DataChannel string                         `json:"dataChannel,omitempty"`
	ICEServers  []RemoteDesktopWebRTCICEServer `json:"iceServers,omitempty"`
}

type RemoteDesktopWebRTCAnswer struct {
	Answer      string                         `json:"answer"`
	DataChannel string                         `json:"dataChannel,omitempty"`
	ICEServers  []RemoteDesktopWebRTCICEServer `json:"iceServers,omitempty"`
}

type RemoteDesktopWebRTCICEServer struct {
	URLs           []string `json:"urls"`
	Username       string   `json:"username,omitempty"`
	Credential     string   `json:"credential,omitempty"`
	CredentialType string   `json:"credentialType,omitempty"`
}

type RemoteDesktopVideoClip struct {
	DurationMs int                      `json:"durationMs" msgpack:"durationMs"`
	Frames     []RemoteDesktopClipFrame `json:"frames" msgpack:"frames"`
}

type RemoteDesktopClipFrame struct {
	OffsetMs int    `json:"offsetMs" msgpack:"offsetMs"`
	Width    int    `json:"width" msgpack:"width"`
	Height   int    `json:"height" msgpack:"height"`
	Encoding string `json:"encoding" msgpack:"encoding"`
	Data     []byte `json:"data" msgpack:"data"`
}

type RemoteDesktopSession struct {
	ID                 string
	Settings           RemoteDesktopSettings
	ActiveEncoder      RemoteDesktopEncoder
	NegotiatedCodec    RemoteDesktopEncoder
	Transport          RemoteDesktopTransport
	IntraRefresh       bool
	EncoderHardware    string
	TransportFeatures  map[string]bool
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
	transport          frameTransport
	inputBridge        *quicInputBridge
}

type remoteMonitor struct {
	info   RemoteDesktopMonitorInfo
	bounds image.Rectangle
}

type Engine interface {
	Configure(Config) error
	StartSession(context.Context, RemoteDesktopCommandPayload) error
	StopSession(string) error
	UpdateSession(RemoteDesktopCommandPayload) error
	HandleInput(context.Context, RemoteDesktopCommandPayload) error
	DeliverFrame(context.Context, RemoteDesktopFramePacket) error
	Shutdown()
}

type RemoteDesktopStreamer struct {
	controller *remoteDesktopSessionController
}

var _ Engine = (*RemoteDesktopStreamer)(nil)

type remoteDesktopSessionController struct {
	cfg            atomic.Value // stores Config
	mu             sync.Mutex
	session        *RemoteDesktopSession
	endpointCache  atomic.Value
	transportCache atomic.Value
}
