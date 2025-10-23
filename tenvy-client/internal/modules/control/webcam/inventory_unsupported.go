//go:build !linux && !windows && !darwin

package webcam

import "github.com/rootbay/tenvy-client/internal/protocol"

func platformCaptureWebcamInventory() ([]protocol.WebcamDevice, string, error) {
	return []protocol.WebcamDevice{}, "webcam enumeration is not implemented on this platform", nil
}
