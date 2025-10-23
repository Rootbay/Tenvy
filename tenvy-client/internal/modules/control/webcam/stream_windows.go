//go:build windows

package webcam

import (
	"fmt"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

func defaultFrameSourceFactory(deviceID string, settings *protocol.WebcamStreamSettings) (frameSource, error) {
	return newNativeFrameSource(deviceID, settings, nativeFrameConfigurator{
		defaultMimeType:    "image/jpeg",
		defaultPixelFormat: "NV12",
		fallbackFrameRate:  30,
		frameGenerator: func(deviceID string) func() []byte {
			return func() []byte {
				payload := fmt.Sprintf("mf:%s:%d", deviceID, time.Now().UnixNano())
				return []byte(payload)
			}
		},
	})
}
