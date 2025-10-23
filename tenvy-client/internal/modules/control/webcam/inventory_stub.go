package webcam

import "github.com/rootbay/tenvy-client/internal/protocol"

func captureWebcamInventory() ([]protocol.WebcamDevice, string, error) {
	devices := make([]protocol.WebcamDevice, 0)
	warning := "webcam enumeration is not implemented on this platform"
	return devices, warning, nil
}
