//go:build windows

package webcam

import (
	"sort"
	"strings"

	"github.com/rootbay/tenvy-client/internal/protocol"
	"github.com/yusufpapurcu/wmi"
)

type win32PnPEntity struct {
	DeviceID string
	Name     string
	PNPClass string
	Service  string
}

func platformCaptureWebcamInventory() ([]protocol.WebcamDevice, string, error) {
	var entities []win32PnPEntity
	query := `SELECT DeviceID, Name, PNPClass, Service FROM Win32_PnPEntity WHERE PNPClass = 'Camera' OR Service = 'usbvideo'`
	if err := wmi.Query(query, &entities); err != nil {
		return nil, "", err
	}

	devices := make([]protocol.WebcamDevice, 0, len(entities))
	for _, entity := range entities {
		id := strings.TrimSpace(entity.DeviceID)
		name := strings.TrimSpace(entity.Name)
		if id == "" && name == "" {
			continue
		}
		if id == "" {
			id = name
		}
		if name == "" {
			name = id
		}

		devices = append(devices, protocol.WebcamDevice{ID: id, Label: name})
	}

	sort.SliceStable(devices, func(i, j int) bool {
		if devices[i].Label == devices[j].Label {
			return devices[i].ID < devices[j].ID
		}
		return devices[i].Label < devices[j].Label
	})

	return devices, "", nil
}
