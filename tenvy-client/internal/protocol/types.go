package protocol

import (
	"encoding/json"
	"errors"
	"strings"

	manifest "github.com/rootbay/tenvy-client/shared/pluginmanifest"
)

const (
	CommandStreamSubprotocol    = "tenvy.agent.v1"
	CommandStreamMaxMessageSize = 1 << 20 // 1 MiB
	AudioStreamSubprotocol      = "tenvy.audio.v1"
	AudioStreamTokenHeader      = "X-Audio-Stream-Token"
)

var ErrUnauthorized = errors.New("unauthorized")

type PluginSignaturePolicy struct {
	SHA256AllowList   []string          `json:"sha256AllowList,omitempty"`
	Ed25519PublicKeys map[string]string `json:"ed25519PublicKeys,omitempty"`
	MaxSignatureAgeMs int64             `json:"maxSignatureAgeMs,omitempty"`
}

type PluginConfig struct {
	SignaturePolicy *PluginSignaturePolicy `json:"signaturePolicy,omitempty"`
}

type AgentConfig struct {
	PollIntervalMs int           `json:"pollIntervalMs"`
	MaxBackoffMs   int           `json:"maxBackoffMs"`
	JitterRatio    float64       `json:"jitterRatio"`
	Plugins        *PluginConfig `json:"plugins,omitempty"`
}

type AgentMetrics struct {
	MemoryBytes   uint64 `json:"memoryBytes,omitempty"`
	Goroutines    int    `json:"goroutines,omitempty"`
	UptimeSeconds uint64 `json:"uptimeSeconds,omitempty"`
}

type Command struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt string          `json:"createdAt"`
}

type CommandEnvelope struct {
	Type        string                   `json:"type"`
	Command     *Command                 `json:"command,omitempty"`
	Input       *RemoteDesktopInputBurst `json:"-"`
	AppVncInput *AppVncInputBurst        `json:"-"`
}

type commandEnvelopeAlias struct {
	Type    string          `json:"type"`
	Command *Command        `json:"command,omitempty"`
	Input   json.RawMessage `json:"input,omitempty"`
}

func (e CommandEnvelope) MarshalJSON() ([]byte, error) {
	alias := commandEnvelopeAlias{
		Type:    e.Type,
		Command: e.Command,
	}

	switch strings.ToLower(strings.TrimSpace(e.Type)) {
	case "remote-desktop-input":
		if e.Input != nil {
			data, err := json.Marshal(e.Input)
			if err != nil {
				return nil, err
			}
			alias.Input = data
		}
	case "app-vnc-input":
		if e.AppVncInput != nil {
			data, err := json.Marshal(e.AppVncInput)
			if err != nil {
				return nil, err
			}
			alias.Input = data
		}
	default:
		switch {
		case e.Input != nil:
			data, err := json.Marshal(e.Input)
			if err != nil {
				return nil, err
			}
			alias.Input = data
		case e.AppVncInput != nil:
			data, err := json.Marshal(e.AppVncInput)
			if err != nil {
				return nil, err
			}
			alias.Input = data
		}
	}

	return json.Marshal(alias)
}

func (e *CommandEnvelope) UnmarshalJSON(data []byte) error {
	if e == nil {
		return errors.New("command envelope not initialized")
	}

	var alias commandEnvelopeAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}

	e.Type = alias.Type
	e.Command = alias.Command
	e.Input = nil
	e.AppVncInput = nil

	if len(alias.Input) == 0 {
		return nil
	}

	switch strings.ToLower(strings.TrimSpace(alias.Type)) {
	case "remote-desktop-input":
		var burst RemoteDesktopInputBurst
		if err := json.Unmarshal(alias.Input, &burst); err != nil {
			return err
		}
		e.Input = &burst
	case "app-vnc-input":
		var burst AppVncInputBurst
		if err := json.Unmarshal(alias.Input, &burst); err != nil {
			return err
		}
		e.AppVncInput = &burst
	default:
		// Attempt remote desktop decoding first for backwards compatibility.
		var remote RemoteDesktopInputBurst
		if err := json.Unmarshal(alias.Input, &remote); err == nil {
			e.Input = &remote
			return nil
		}
		var appBurst AppVncInputBurst
		if err := json.Unmarshal(alias.Input, &appBurst); err == nil {
			e.AppVncInput = &appBurst
			return nil
		}
		return errors.New("unrecognized command envelope input payload")
	}

	return nil
}

type CommandResult struct {
	CommandID   string `json:"commandId"`
	Success     bool   `json:"success"`
	Output      string `json:"output,omitempty"`
	Error       string `json:"error,omitempty"`
	CompletedAt string `json:"completedAt"`
}

type CommandOutputEvent struct {
	Type      string         `json:"type"`
	CommandID string         `json:"commandId"`
	Sequence  int64          `json:"sequence,omitempty"`
	Data      string         `json:"data,omitempty"`
	Timestamp string         `json:"timestamp"`
	Result    *CommandResult `json:"result,omitempty"`
}

type RemoteDesktopInputType string

const (
	RemoteDesktopInputMouseMove   RemoteDesktopInputType = "mouse-move"
	RemoteDesktopInputMouseButton RemoteDesktopInputType = "mouse-button"
	RemoteDesktopInputMouseScroll RemoteDesktopInputType = "mouse-scroll"
	RemoteDesktopInputKey         RemoteDesktopInputType = "key"
)

type RemoteDesktopInputEvent struct {
	Type       RemoteDesktopInputType `json:"type"`
	CapturedAt int64                  `json:"capturedAt"`
	X          float64                `json:"x,omitempty"`
	Y          float64                `json:"y,omitempty"`
	Normalized bool                   `json:"normalized,omitempty"`
	Monitor    *int                   `json:"monitor,omitempty"`
	Button     string                 `json:"button,omitempty"`
	Pressed    bool                   `json:"pressed,omitempty"`
	DeltaX     float64                `json:"deltaX,omitempty"`
	DeltaY     float64                `json:"deltaY,omitempty"`
	DeltaMode  int                    `json:"deltaMode,omitempty"`
	Key        string                 `json:"key,omitempty"`
	Code       string                 `json:"code,omitempty"`
	KeyCode    int                    `json:"keyCode,omitempty"`
	Repeat     bool                   `json:"repeat,omitempty"`
	AltKey     bool                   `json:"altKey,omitempty"`
	CtrlKey    bool                   `json:"ctrlKey,omitempty"`
	ShiftKey   bool                   `json:"shiftKey,omitempty"`
	MetaKey    bool                   `json:"metaKey,omitempty"`
}

type RemoteDesktopInputBurst struct {
	SessionID string                    `json:"sessionId"`
	Sequence  int64                     `json:"sequence,omitempty"`
	Events    []RemoteDesktopInputEvent `json:"events"`
}

type WebcamQuality string

const (
	WebcamQualityMax    WebcamQuality = "max"
	WebcamQualityHigh   WebcamQuality = "high"
	WebcamQualityMedium WebcamQuality = "medium"
	WebcamQualityLow    WebcamQuality = "low"
)

type WebcamResolution struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type WebcamZoomRange struct {
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
	Step float64 `json:"step"`
}

type WebcamDeviceCapabilities struct {
	Resolutions []WebcamResolution `json:"resolutions,omitempty"`
	FrameRates  []float64          `json:"frameRates,omitempty"`
	Zoom        *WebcamZoomRange   `json:"zoom,omitempty"`
	FacingMode  string             `json:"facingMode,omitempty"`
}

type WebcamDevice struct {
	ID           string                    `json:"id"`
	Label        string                    `json:"label"`
	Capabilities *WebcamDeviceCapabilities `json:"capabilities,omitempty"`
}

type WebcamDeviceInventory struct {
	Devices    []WebcamDevice `json:"devices"`
	CapturedAt string         `json:"capturedAt"`
	RequestID  string         `json:"requestId,omitempty"`
	Warning    string         `json:"warning,omitempty"`
}

type WebcamStreamSettings struct {
	Quality     WebcamQuality `json:"quality,omitempty"`
	Width       int           `json:"width,omitempty"`
	Height      int           `json:"height,omitempty"`
	FrameRate   float64       `json:"frameRate,omitempty"`
	Zoom        float64       `json:"zoom,omitempty"`
	MimeType    string        `json:"mimeType,omitempty"`
	PixelFormat string        `json:"pixelFormat,omitempty"`
}

type WebcamNegotiationOffer struct {
	Transport   string   `json:"transport"`
	Offer       string   `json:"offer,omitempty"`
	IceServers  []string `json:"iceServers,omitempty"`
	DataChannel string   `json:"dataChannel,omitempty"`
}

type WebcamNegotiationAnswer struct {
	Answer      string   `json:"answer,omitempty"`
	IceServers  []string `json:"iceServers,omitempty"`
	DataChannel string   `json:"dataChannel,omitempty"`
}

type WebcamNegotiationState struct {
	Offer  *WebcamNegotiationOffer  `json:"offer,omitempty"`
	Answer *WebcamNegotiationAnswer `json:"answer,omitempty"`
}

type WebcamCommandPayload struct {
	Action      string                  `json:"action"`
	RequestID   string                  `json:"requestId,omitempty"`
	SessionID   string                  `json:"sessionId,omitempty"`
	DeviceID    string                  `json:"deviceId,omitempty"`
	Settings    *WebcamStreamSettings   `json:"settings,omitempty"`
	Negotiation *WebcamNegotiationState `json:"negotiation,omitempty"`
}

type AppVncInputBurst struct {
	SessionID string             `json:"sessionId"`
	Events    []AppVncInputEvent `json:"events"`
	Sequence  int64              `json:"sequence,omitempty"`
}

type AppVncQuality string

const (
	AppVncQualityLossless  AppVncQuality = "lossless"
	AppVncQualityBalanced  AppVncQuality = "balanced"
	AppVncQualityBandwidth AppVncQuality = "bandwidth"
)

type AppVncPlatform string

const (
	AppVncPlatformWindows AppVncPlatform = "windows"
	AppVncPlatformLinux   AppVncPlatform = "linux"
	AppVncPlatformMacOS   AppVncPlatform = "macos"
)

type AppVncSessionSettings struct {
	Monitor           string        `json:"monitor"`
	Quality           AppVncQuality `json:"quality"`
	CaptureCursor     bool          `json:"captureCursor"`
	ClipboardSync     bool          `json:"clipboardSync"`
	BlockLocalInput   bool          `json:"blockLocalInput"`
	HeartbeatInterval int           `json:"heartbeatInterval"`
	AppID             string        `json:"appId,omitempty"`
	WindowTitle       string        `json:"windowTitle,omitempty"`
}

type AppVncSessionSettingsPatch struct {
	Monitor           *string        `json:"monitor,omitempty"`
	Quality           *AppVncQuality `json:"quality,omitempty"`
	CaptureCursor     *bool          `json:"captureCursor,omitempty"`
	ClipboardSync     *bool          `json:"clipboardSync,omitempty"`
	BlockLocalInput   *bool          `json:"blockLocalInput,omitempty"`
	HeartbeatInterval *int           `json:"heartbeatInterval,omitempty"`
	AppID             *string        `json:"appId,omitempty"`
	WindowTitle       *string        `json:"windowTitle,omitempty"`
}

type AppVncVirtualizationHints struct {
	ProfileSeeds map[AppVncPlatform]string            `json:"profileSeeds,omitempty"`
	DataRoots    map[AppVncPlatform]string            `json:"dataRoots,omitempty"`
	Environment  map[AppVncPlatform]map[string]string `json:"environment,omitempty"`
}

type AppVncVirtualizationPlan struct {
	Platform    AppVncPlatform    `json:"platform,omitempty"`
	ProfileSeed string            `json:"profileSeed,omitempty"`
	DataRoot    string            `json:"dataRoot,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
}

type AppVncApplicationDescriptor struct {
	ID              string                     `json:"id"`
	Name            string                     `json:"name"`
	Summary         string                     `json:"summary"`
	Category        string                     `json:"category"`
	Platforms       []AppVncPlatform           `json:"platforms"`
	WindowTitleHint string                     `json:"windowTitleHint,omitempty"`
	Executable      map[AppVncPlatform]string  `json:"executable,omitempty"`
	Virtualization  *AppVncVirtualizationHints `json:"virtualization,omitempty"`
}

type AppVncPointerButton string

const (
	AppVncPointerButtonLeft   AppVncPointerButton = "left"
	AppVncPointerButtonMiddle AppVncPointerButton = "middle"
	AppVncPointerButtonRight  AppVncPointerButton = "right"
)

type AppVncInputEventType string

const (
	AppVncInputPointerMove   AppVncInputEventType = "pointer-move"
	AppVncInputPointerButton AppVncInputEventType = "pointer-button"
	AppVncInputPointerScroll AppVncInputEventType = "pointer-scroll"
	AppVncInputKey           AppVncInputEventType = "key"
)

type AppVncInputEvent struct {
	Type       AppVncInputEventType `json:"type"`
	CapturedAt int64                `json:"capturedAt"`
	X          float64              `json:"x,omitempty"`
	Y          float64              `json:"y,omitempty"`
	Normalized bool                 `json:"normalized,omitempty"`
	Button     AppVncPointerButton  `json:"button,omitempty"`
	Pressed    bool                 `json:"pressed,omitempty"`
	DeltaX     float64              `json:"deltaX,omitempty"`
	DeltaY     float64              `json:"deltaY,omitempty"`
	DeltaMode  int                  `json:"deltaMode,omitempty"`
	Key        string               `json:"key,omitempty"`
	Code       string               `json:"code,omitempty"`
	KeyCode    int                  `json:"keyCode,omitempty"`
	Repeat     bool                 `json:"repeat,omitempty"`
	AltKey     bool                 `json:"altKey,omitempty"`
	CtrlKey    bool                 `json:"ctrlKey,omitempty"`
	ShiftKey   bool                 `json:"shiftKey,omitempty"`
	MetaKey    bool                 `json:"metaKey,omitempty"`
}

type AppVncCommandPayload struct {
	Action         string                       `json:"action"`
	SessionID      string                       `json:"sessionId,omitempty"`
	Settings       *AppVncSessionSettingsPatch  `json:"settings,omitempty"`
	Events         []AppVncInputEvent           `json:"events,omitempty"`
	Application    *AppVncApplicationDescriptor `json:"application,omitempty"`
	Virtualization *AppVncVirtualizationPlan    `json:"virtualization,omitempty"`
}

type AgentMetadata struct {
	Hostname        string   `json:"hostname"`
	Username        string   `json:"username"`
	OS              string   `json:"os"`
	Architecture    string   `json:"architecture"`
	IPAddress       string   `json:"ipAddress,omitempty"`
	PublicIPAddress string   `json:"publicIpAddress,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	Version         string   `json:"version,omitempty"`
}

type AgentRegistrationRequest struct {
	Token    string        `json:"token,omitempty"`
	Metadata AgentMetadata `json:"metadata"`
}

type AgentRegistrationResponse struct {
	AgentID    string      `json:"agentId"`
	AgentKey   string      `json:"agentKey"`
	Config     AgentConfig `json:"config"`
	Commands   []Command   `json:"commands"`
	ServerTime string      `json:"serverTime"`
}

type AgentSyncRequest struct {
	Status    string                `json:"status"`
	Timestamp string                `json:"timestamp"`
	Metrics   *AgentMetrics         `json:"metrics,omitempty"`
	Results   []CommandResult       `json:"results,omitempty"`
	Plugins   *manifest.SyncPayload `json:"plugins,omitempty"`
}

type AgentSyncResponse struct {
	AgentID         string                  `json:"agentId"`
	Commands        []Command               `json:"commands"`
	Config          AgentConfig             `json:"config"`
	ServerTime      string                  `json:"serverTime"`
	PluginManifests *manifest.ManifestDelta `json:"pluginManifests,omitempty"`
}

type PingCommandPayload struct {
	Message string `json:"message,omitempty"`
}

type ShellCommandPayload struct {
	Command          string            `json:"command"`
	TimeoutSeconds   int               `json:"timeoutSeconds,omitempty"`
	WorkingDirectory string            `json:"workingDirectory,omitempty"`
	Elevated         bool              `json:"elevated,omitempty"`
	Environment      map[string]string `json:"environment,omitempty"`
}

type OpenURLCommandPayload struct {
	URL  string `json:"url"`
	Note string `json:"note,omitempty"`
}

type AgentControlCommandPayload struct {
	Action string `json:"action"`
	Reason string `json:"reason,omitempty"`
}

type ToolActivationCommandPayload struct {
	ToolID      string         `json:"toolId"`
	Action      string         `json:"action"`
	InitiatedBy string         `json:"initiatedBy,omitempty"`
	Timestamp   string         `json:"timestamp,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type RecoveryTargetSelection struct {
	Type      string   `json:"type"`
	Label     string   `json:"label,omitempty"`
	Path      string   `json:"path,omitempty"`
	Paths     []string `json:"paths,omitempty"`
	Recursive bool     `json:"recursive,omitempty"`
}

type RecoveryCommandPayload struct {
	RequestID   string                    `json:"requestId"`
	Selections  []RecoveryTargetSelection `json:"selections"`
	ArchiveName string                    `json:"archiveName,omitempty"`
	Notes       string                    `json:"notes,omitempty"`
}

type RecoveryManifestEntry struct {
	Path            string `json:"path"`
	Size            int64  `json:"size"`
	ModifiedAt      string `json:"modifiedAt"`
	Mode            string `json:"mode"`
	Type            string `json:"type"`
	Target          string `json:"target"`
	SourcePath      string `json:"sourcePath,omitempty"`
	Preview         string `json:"preview,omitempty"`
	PreviewEncoding string `json:"previewEncoding,omitempty"`
	Truncated       bool   `json:"truncated,omitempty"`
}

type ClientChatAliasConfiguration struct {
	Operator string `json:"operator,omitempty"`
	Client   string `json:"client,omitempty"`
}

type ClientChatFeatureFlags struct {
	Unstoppable        *bool `json:"unstoppable,omitempty"`
	AllowNotifications *bool `json:"allowNotifications,omitempty"`
	AllowFileTransfers *bool `json:"allowFileTransfers,omitempty"`
}

type ClientChatCommandMessage struct {
	ID        string `json:"id,omitempty"`
	Body      string `json:"body"`
	Timestamp string `json:"timestamp,omitempty"`
	Alias     string `json:"alias,omitempty"`
}

type ClientChatCommandPayload struct {
	Action    string                        `json:"action"`
	SessionID string                        `json:"sessionId,omitempty"`
	Message   *ClientChatCommandMessage     `json:"message,omitempty"`
	Aliases   *ClientChatAliasConfiguration `json:"aliases,omitempty"`
	Features  *ClientChatFeatureFlags       `json:"features,omitempty"`
}

type ClientChatMessage struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	Timestamp string `json:"timestamp"`
	Alias     string `json:"alias,omitempty"`
}

type ClientChatMessageEnvelope struct {
	SessionID string            `json:"sessionId"`
	Message   ClientChatMessage `json:"message"`
}
