//go:build !cgo
// +build !cgo

package audio

import (
	"context"
	"net/http"
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
	AgentID   string
	BaseURL   string
	AuthKey   string
	Client    HTTPDoer
	Logger    Logger
	UserAgent string
}

type AudioBridge struct {
	cfg Config
}

func NewAudioBridge(cfg Config) *AudioBridge {
	return &AudioBridge{cfg: cfg}
}

func (b *AudioBridge) Shutdown() {}

func (b *AudioBridge) HandleCommand(ctx context.Context, cmd Command) CommandResult {
	_ = ctx

	return CommandResult{
		CommandID:   cmd.ID,
		Success:     false,
		Error:       "audio-control unavailable: client built without cgo/audio support",
		CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
}
