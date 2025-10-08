//go:build windows

package remotedesktop

import (
	"image"
	"math"
	"strings"
	"unsafe"

	"github.com/lxn/win"
)

const wheelDelta = 120

func processRemoteInput(monitors []remoteMonitor, settings RemoteDesktopSettings, events []RemoteDesktopInputEvent) error {
	fallback := selectMonitorForInput(monitors, settings.Monitor)
	for _, event := range events {
		switch event.Type {
		case RemoteInputMouseMove:
			target := monitorFromEvent(monitors, fallback, event.Monitor)
			moveMouse(event, target)
		case RemoteInputMouseButton:
			sendMouseButton(event.Button, event.Pressed)
		case RemoteInputMouseScroll:
			sendMouseScroll(event)
		case RemoteInputKey:
			sendKeyEvent(event)
		}
	}
	return nil
}

func selectMonitorForInput(monitors []remoteMonitor, index int) remoteMonitor {
	if len(monitors) == 0 {
		bounds := virtualScreenBounds()
		info := RemoteDesktopMonitorInfo{Width: bounds.Dx(), Height: bounds.Dy()}
		return remoteMonitor{info: info, bounds: bounds}
	}
	if index < 0 || index >= len(monitors) {
		index = 0
	}
	return monitors[index]
}

func monitorFromEvent(monitors []remoteMonitor, fallback remoteMonitor, override *int) remoteMonitor {
	if override != nil {
		idx := *override
		if idx >= 0 && idx < len(monitors) {
			return monitors[idx]
		}
	}
	return fallback
}

func virtualScreenBounds() image.Rectangle {
	width := int(win.GetSystemMetrics(win.SM_CXVIRTUALSCREEN))
	height := int(win.GetSystemMetrics(win.SM_CYVIRTUALSCREEN))
	originX := int(win.GetSystemMetrics(win.SM_XVIRTUALSCREEN))
	originY := int(win.GetSystemMetrics(win.SM_YVIRTUALSCREEN))
	return image.Rect(originX, originY, originX+width, originY+height)
}

func moveMouse(event RemoteDesktopInputEvent, monitor remoteMonitor) {
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

	win.SetCursorPos(int32(math.Round(targetX)), int32(math.Round(targetY)))
}

func sendMouseButton(button RemoteDesktopMouseButton, pressed bool) {
	var flag uint32
	switch button {
	case RemoteMouseButtonLeft:
		if pressed {
			flag = win.MOUSEEVENTF_LEFTDOWN
		} else {
			flag = win.MOUSEEVENTF_LEFTUP
		}
	case RemoteMouseButtonRight:
		if pressed {
			flag = win.MOUSEEVENTF_RIGHTDOWN
		} else {
			flag = win.MOUSEEVENTF_RIGHTUP
		}
	case RemoteMouseButtonMiddle:
		if pressed {
			flag = win.MOUSEEVENTF_MIDDLEDOWN
		} else {
			flag = win.MOUSEEVENTF_MIDDLEUP
		}
	default:
		return
	}

	input := win.MOUSE_INPUT{
		Type: win.INPUT_MOUSE,
		Mi: win.MOUSEINPUT{
			DwFlags: flag,
		},
	}
	win.SendInput(1, unsafe.Pointer(&input), int32(unsafe.Sizeof(input)))
}

func sendMouseScroll(event RemoteDesktopInputEvent) {
	inputs := make([]win.MOUSE_INPUT, 0, 2)
	if event.DeltaY != 0 {
		amount := scaledWheelAmount(-event.DeltaY, event.DeltaMode)
		if amount != 0 {
			inputs = append(inputs, win.MOUSE_INPUT{
				Type: win.INPUT_MOUSE,
				Mi: win.MOUSEINPUT{
					DwFlags:   win.MOUSEEVENTF_WHEEL,
					MouseData: uint32(amount),
				},
			})
		}
	}
	if event.DeltaX != 0 {
		amount := scaledWheelAmount(event.DeltaX, event.DeltaMode)
		if amount != 0 {
			inputs = append(inputs, win.MOUSE_INPUT{
				Type: win.INPUT_MOUSE,
				Mi: win.MOUSEINPUT{
					DwFlags:   win.MOUSEEVENTF_HWHEEL,
					MouseData: uint32(amount),
				},
			})
		}
	}
	if len(inputs) == 0 {
		return
	}
	win.SendInput(uint32(len(inputs)), unsafe.Pointer(&inputs[0]), int32(unsafe.Sizeof(inputs[0])))
}

func scaledWheelAmount(delta float64, mode int) int32 {
	if delta == 0 {
		return 0
	}

	scale := 1.0
	switch mode {
	case 0: // pixel
		scale = 1.0 / 32.0
	case 1: // line
		scale = 1.0
	case 2: // page
		scale = 4.0
	}

	amount := int32(math.Round(delta * scale))
	if amount == 0 {
		if delta > 0 {
			amount = 1
		} else {
			amount = -1
		}
	}
	return amount * wheelDelta
}

func sendKeyEvent(event RemoteDesktopInputEvent) {
	code, ok := resolveVirtualKey(event)
	if !ok {
		return
	}

	flags := uint32(0)
	if !event.Pressed {
		flags |= win.KEYEVENTF_KEYUP
	}
	if isExtendedKey(code) {
		flags |= win.KEYEVENTF_EXTENDEDKEY
	}

	input := win.KEYBD_INPUT{
		Type: win.INPUT_KEYBOARD,
		Ki: win.KEYBDINPUT{
			WVk:     code,
			DwFlags: flags,
		},
	}

	win.SendInput(1, unsafe.Pointer(&input), int32(unsafe.Sizeof(input)))
}

func resolveVirtualKey(event RemoteDesktopInputEvent) (uint16, bool) {
	code := event.KeyCode
	if code == 0 && len(event.Key) == 1 {
		code = int(strings.ToUpper(event.Key)[0])
	}
	if code <= 0 {
		return 0, false
	}
	return uint16(code), true
}

func isExtendedKey(code uint16) bool {
	switch code {
	case win.VK_RMENU, win.VK_RCONTROL, win.VK_INSERT, win.VK_DELETE, win.VK_HOME, win.VK_END,
		win.VK_PRIOR, win.VK_NEXT, win.VK_LEFT, win.VK_UP, win.VK_RIGHT, win.VK_DOWN,
		win.VK_NUMLOCK, win.VK_CANCEL, win.VK_SNAPSHOT, win.VK_DIVIDE:
		return true
	default:
		return false
	}
}
