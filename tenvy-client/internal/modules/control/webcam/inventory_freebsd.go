//go:build freebsd

package webcam

import "github.com/rootbay/tenvy-client/internal/protocol"

func platformCaptureWebcamInventory() ([]protocol.WebcamDevice, string, error) {
	return captureV4L2WebcamInventory()
}
