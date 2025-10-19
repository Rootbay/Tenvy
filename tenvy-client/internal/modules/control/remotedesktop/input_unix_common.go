//go:build linux || darwin

package remotedesktop

import "image"

func resolvePointerPosition(event RemoteDesktopInputEvent, monitor remoteMonitor) (float64, float64) {
	bounds := monitor.bounds
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		width = maxInt(monitor.info.Width, 1)
		height = maxInt(monitor.info.Height, 1)
		bounds = image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+width, bounds.Min.Y+height)
	}
	normX := event.X
	normY := event.Y
	if event.Normalized {
		normX = clampFloat(normX, 0, 1)
		normY = clampFloat(normY, 0, 1)
	} else {
		normX = clampFloat(normX/float64(maxInt(monitor.info.Width-1, 1)), 0, 1)
		normY = clampFloat(normY/float64(maxInt(monitor.info.Height-1, 1)), 0, 1)
	}
	targetX := float64(bounds.Min.X)
	targetY := float64(bounds.Min.Y)
	if width > 1 {
		targetX += normX * float64(width-1)
	}
	if height > 1 {
		targetY += normY * float64(height-1)
	}
	return targetX, targetY
}
