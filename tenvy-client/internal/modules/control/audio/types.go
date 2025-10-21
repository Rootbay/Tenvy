package audio

import "time"

type AudioDirection string

const (
	AudioDirectionInput  AudioDirection = "input"
	AudioDirectionOutput AudioDirection = "output"
)

type AudioDeviceDescriptor struct {
	ID                    string         `json:"id"`
	DeviceID              string         `json:"deviceId"`
	Label                 string         `json:"label"`
	Kind                  AudioDirection `json:"kind"`
	GroupID               string         `json:"groupId"`
	SystemDefault         bool           `json:"systemDefault"`
	CommunicationsDefault bool           `json:"communicationsDefault"`
	LastSeen              string         `json:"lastSeen"`
}

type AudioDeviceInventory struct {
	Inputs     []AudioDeviceDescriptor `json:"inputs"`
	Outputs    []AudioDeviceDescriptor `json:"outputs"`
	CapturedAt string                  `json:"capturedAt"`
	RequestID  string                  `json:"requestId,omitempty"`
}

type AudioStreamFormat struct {
	Encoding   string `json:"encoding"`
	SampleRate int    `json:"sampleRate"`
	Channels   int    `json:"channels"`
}

type AudioStreamTransport struct {
	Transport string            `json:"transport"`
	URL       string            `json:"url"`
	Protocol  string            `json:"protocol,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

type AudioStreamChunk struct {
	SessionID string            `json:"sessionId"`
	Sequence  uint64            `json:"sequence"`
	Timestamp string            `json:"timestamp"`
	Format    AudioStreamFormat `json:"format"`
	Data      string            `json:"data"`
}

type AudioControlCommandPayload struct {
	Action          string                `json:"action"`
	RequestID       string                `json:"requestId,omitempty"`
	SessionID       string                `json:"sessionId,omitempty"`
	DeviceID        string                `json:"deviceId,omitempty"`
	DeviceLabel     string                `json:"deviceLabel,omitempty"`
	Direction       AudioDirection        `json:"direction,omitempty"`
	SampleRate      int                   `json:"sampleRate,omitempty"`
	Channels        int                   `json:"channels,omitempty"`
	Encoding        string                `json:"encoding,omitempty"`
	StreamTransport *AudioStreamTransport `json:"streamTransport,omitempty"`
}

type AudioDiagnosticResult struct {
	Inventory     *AudioDeviceInventory
	BytesCaptured uint64
	Duration      time.Duration
}
