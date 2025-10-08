//go:build !cgo
// +build !cgo

package main

import (
	"context"
	"time"
)

type AudioBridge struct {
	agent *Agent
}

func NewAudioBridge(agent *Agent) *AudioBridge {
	return &AudioBridge{agent: agent}
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
