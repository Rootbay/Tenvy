//go:build darwin

package remotedesktopengine

/*
#cgo CFLAGS: -x objective-c -fmodules -fobjc-arc
#cgo LDFLAGS: -framework ApplicationServices -framework CoreGraphics
#include <ApplicationServices/ApplicationServices.h>
#include <CoreGraphics/CoreGraphics.h>
#include <math.h>
#include <stdbool.h>

static CGPoint rd_current_position(void) {
        CGEventRef event = CGEventCreate(NULL);
        CGPoint location = CGEventGetLocation(event);
        CFRelease(event);
        return location;
}

static void rd_move_mouse(CGFloat x, CGFloat y) {
        CGWarpMouseCursorPosition(CGPointMake(x, y));
        CGAssociateMouseAndMouseCursorPosition(true);
}

static void rd_mouse_button(int button, bool down) {
        CGPoint location = rd_current_position();
        CGMouseButton cgButton = kCGMouseButtonLeft;
        CGEventType typeDown = kCGEventLeftMouseDown;
        CGEventType typeUp = kCGEventLeftMouseUp;
        switch (button) {
        case 0:
                cgButton = kCGMouseButtonLeft;
                typeDown = kCGEventLeftMouseDown;
                typeUp = kCGEventLeftMouseUp;
                break;
        case 1:
                cgButton = kCGMouseButtonRight;
                typeDown = kCGEventRightMouseDown;
                typeUp = kCGEventRightMouseUp;
                break;
        case 2:
                cgButton = kCGMouseButtonCenter;
                typeDown = kCGEventOtherMouseDown;
                typeUp = kCGEventOtherMouseUp;
                break;
        default:
                return;
        }
        CGEventType type = down ? typeDown : typeUp;
        CGEventRef event = CGEventCreateMouseEvent(NULL, type, location, cgButton);
        CGEventPost(kCGHIDEventTap, event);
        CFRelease(event);
}

static void rd_scroll(double dx, double dy, int mode) {
        CGScrollEventUnit unit = kCGScrollEventUnitLine;
        double scale = 1.0;
        if (mode == 0) {
                unit = kCGScrollEventUnitPixel;
                scale = 1.0;
        } else if (mode == 2) {
                scale = 3.0;
        }
        double xAmount = dx * scale;
        double yAmount = dy * scale;
        int32_t yValue = (int32_t)lround(-yAmount);
        int32_t xValue = (int32_t)lround(xAmount);
        CGEventRef event = CGEventCreateScrollWheelEvent(NULL, unit, 2, yValue, xValue);
        CGEventPost(kCGHIDEventTap, event);
        CFRelease(event);
}

static void rd_key_event(CGKeyCode code, bool down) {
        CGEventRef event = CGEventCreateKeyboardEvent(NULL, code, down);
        CGEventPost(kCGHIDEventTap, event);
        CFRelease(event);
}

static void rd_unicode_event(uint32_t rune, bool down) {
        UniChar chars[2];
        size_t length = 0;
        if (rune <= 0xFFFF) {
                chars[0] = (UniChar)rune;
                length = 1;
        } else {
                uint32_t value = rune - 0x10000;
                chars[0] = (UniChar)((value >> 10) + 0xD800);
                chars[1] = (UniChar)((value & 0x3FF) + 0xDC00);
                length = 2;
        }
        CGEventRef event = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)0, down);
        CGEventKeyboardSetUnicodeString(event, length, chars);
        CGEventPost(kCGHIDEventTap, event);
        CFRelease(event);
}

static void rd_main_display_bounds(double *x, double *y, double *w, double *h) {
        CGRect bounds = CGDisplayBounds(CGMainDisplayID());
        if (x) *x = bounds.origin.x;
        if (y) *y = bounds.origin.y;
        if (w) *w = bounds.size.width;
        if (h) *h = bounds.size.height;
}
*/
import "C"

import (
	"image"
	"strings"
	"sync"
	"unicode"
)

type macInput struct {
	mu       sync.Mutex
	fallback remoteMonitor
}

var (
	macOnce     sync.Once
	macInstance *macInput
)

func processRemoteInput(monitors []remoteMonitor, settings RemoteDesktopSettings, events []RemoteDesktopInputEvent) error {
	if len(events) == 0 {
		return nil
	}

	input := macInputInstance()
	fallback := selectMonitorForInputDarwin(monitors, settings.Monitor, input.fallback)

	input.mu.Lock()
	defer input.mu.Unlock()

	for _, event := range events {
		switch event.Type {
		case RemoteInputMouseMove:
			target := monitorFromEventDarwin(monitors, fallback, event.Monitor)
			if err := input.movePointer(event, target); err != nil {
				return err
			}
		case RemoteInputMouseButton:
			if err := input.sendMouseButton(event.Button, event.Pressed); err != nil {
				return err
			}
		case RemoteInputMouseScroll:
			input.sendMouseScroll(event)
		case RemoteInputKey:
			if err := input.sendKeyEvent(event); err != nil {
				return err
			}
		}
	}
	return nil
}

func macInputInstance() *macInput {
	macOnce.Do(func() {
		macInstance = &macInput{fallback: detectMacFallbackMonitor()}
	})
	return macInstance
}

func detectMacFallbackMonitor() remoteMonitor {
	var x, y, w, h C.double
	C.rd_main_display_bounds(&x, &y, &w, &h)
	rect := image.Rect(int(x), int(y), int(x+w), int(y+h))
	info := RemoteDesktopMonitorInfo{Width: rect.Dx(), Height: rect.Dy()}
	return remoteMonitor{info: info, bounds: rect}
}

func selectMonitorForInputDarwin(monitors []remoteMonitor, index int, fallback remoteMonitor) remoteMonitor {
	if len(monitors) == 0 {
		return fallback
	}
	if index < 0 || index >= len(monitors) {
		index = 0
	}
	return monitors[index]
}

func monitorFromEventDarwin(monitors []remoteMonitor, fallback remoteMonitor, override *int) remoteMonitor {
	if override != nil {
		idx := *override
		if idx >= 0 && idx < len(monitors) {
			return monitors[idx]
		}
	}
	return fallback
}

func (m *macInput) movePointer(event RemoteDesktopInputEvent, monitor remoteMonitor) error {
	x, y := resolvePointerPosition(event, monitor)
	C.rd_move_mouse(C.CGFloat(x), C.CGFloat(y))
	return nil
}

func (m *macInput) sendMouseButton(button RemoteDesktopMouseButton, pressed bool) error {
	var code C.int
	switch button {
	case RemoteMouseButtonLeft:
		code = 0
	case RemoteMouseButtonRight:
		code = 1
	case RemoteMouseButtonMiddle:
		code = 2
	default:
		return nil
	}
	down := C.bool(0)
	if pressed {
		down = C.bool(1)
	}
	C.rd_mouse_button(code, down)
	return nil
}

func (m *macInput) sendMouseScroll(event RemoteDesktopInputEvent) {
	C.rd_scroll(C.double(event.DeltaX), C.double(event.DeltaY), C.int(event.DeltaMode))
}

func (m *macInput) sendKeyEvent(event RemoteDesktopInputEvent) error {
	if code, ok := macKeyCodeForEvent(event); ok {
		down := C.bool(0)
		if event.Pressed {
			down = C.bool(1)
		}
		C.rd_key_event(C.CGKeyCode(code), down)
		return nil
	}
	if len(event.Key) == 1 {
		r := []rune(event.Key)[0]
		down := C.bool(0)
		if event.Pressed {
			down = C.bool(1)
		}
		C.rd_unicode_event(C.uint(r), down)
		return nil
	}
	return nil
}

func macKeyCodeForEvent(event RemoteDesktopInputEvent) (uint16, bool) {
	if code, ok := macKeyCodeByCode[event.Code]; ok {
		return code, true
	}
	if code, ok := macKeyCodeByKey[event.Key]; ok {
		return code, true
	}
	if strings.HasPrefix(event.Code, "Key") && len(event.Code) == 4 {
		if code, ok := macRuneKeyCode(rune(event.Code[3])); ok {
			return code, true
		}
	}
	if strings.HasPrefix(event.Code, "Digit") && len(event.Code) == 6 {
		if code, ok := macRuneKeyCode(rune(event.Code[5])); ok {
			return code, true
		}
	}
	if strings.HasPrefix(event.Code, "Numpad") {
		if code, ok := macKeyCodeByCode[event.Code]; ok {
			return code, true
		}
	}
	if len(event.Key) == 1 {
		if code, ok := macRuneKeyCode([]rune(event.Key)[0]); ok {
			return code, true
		}
	}
	return 0, false
}

func macRuneKeyCode(r rune) (uint16, bool) {
	if code, ok := macCharacterKeyCodes[r]; ok {
		return code, true
	}
	upper := unicode.ToUpper(r)
	if code, ok := macCharacterKeyCodes[upper]; ok {
		return code, true
	}
	return 0, false
}

var macCharacterKeyCodes = map[rune]uint16{
	'A':  0x00,
	'S':  0x01,
	'D':  0x02,
	'F':  0x03,
	'H':  0x04,
	'G':  0x05,
	'Z':  0x06,
	'X':  0x07,
	'C':  0x08,
	'V':  0x09,
	'B':  0x0B,
	'Q':  0x0C,
	'W':  0x0D,
	'E':  0x0E,
	'R':  0x0F,
	'Y':  0x10,
	'T':  0x11,
	'1':  0x12,
	'2':  0x13,
	'3':  0x14,
	'4':  0x15,
	'6':  0x16,
	'5':  0x17,
	'=':  0x18,
	'9':  0x19,
	'7':  0x1A,
	'-':  0x1B,
	'8':  0x1C,
	'0':  0x1D,
	']':  0x1E,
	'O':  0x1F,
	'U':  0x20,
	'[':  0x21,
	'I':  0x22,
	'P':  0x23,
	'L':  0x25,
	'J':  0x26,
	'\'': 0x27,
	'K':  0x28,
	';':  0x29,
	'\\': 0x2A,
	',':  0x2B,
	'/':  0x2C,
	'N':  0x2D,
	'M':  0x2E,
	'.':  0x2F,
	'`':  0x32,
	' ':  0x31,
}

var macKeyCodeByKey = map[string]uint16{
	"Backspace":    0x33,
	"Delete":       0x75,
	"Enter":        0x24,
	"Return":       0x24,
	"Tab":          0x30,
	"Escape":       0x35,
	"Space":        0x31,
	" ":            0x31,
	"ArrowUp":      0x7e,
	"ArrowDown":    0x7d,
	"ArrowLeft":    0x7b,
	"ArrowRight":   0x7c,
	"Home":         0x73,
	"End":          0x77,
	"PageUp":       0x74,
	"PageDown":     0x79,
	"Insert":       0x72,
	"CapsLock":     0x39,
	"Shift":        0x38,
	"ShiftLeft":    0x38,
	"ShiftRight":   0x3c,
	"Control":      0x3b,
	"ControlLeft":  0x3b,
	"ControlRight": 0x3e,
	"Alt":          0x3a,
	"AltLeft":      0x3a,
	"AltRight":     0x3d,
	"Meta":         0x37,
	"MetaLeft":     0x37,
	"MetaRight":    0x36,
	"ContextMenu":  0x6e,
	"F1":           0x7a,
	"F2":           0x78,
	"F3":           0x63,
	"F4":           0x76,
	"F5":           0x60,
	"F6":           0x61,
	"F7":           0x62,
	"F8":           0x64,
	"F9":           0x65,
	"F10":          0x6d,
	"F11":          0x67,
	"F12":          0x6f,
}

var macKeyCodeByCode = map[string]uint16{
	"Backspace":      0x33,
	"Delete":         0x75,
	"Enter":          0x24,
	"NumpadEnter":    0x4c,
	"NumpadDivide":   0x4b,
	"NumpadMultiply": 0x43,
	"NumpadSubtract": 0x4e,
	"NumpadAdd":      0x45,
	"NumpadDecimal":  0x41,
	"NumpadEqual":    0x51,
	"Numpad0":        0x52,
	"Numpad1":        0x53,
	"Numpad2":        0x54,
	"Numpad3":        0x55,
	"Numpad4":        0x56,
	"Numpad5":        0x57,
	"Numpad6":        0x58,
	"Numpad7":        0x59,
	"Numpad8":        0x5b,
	"Numpad9":        0x5c,
	"ArrowUp":        0x7e,
	"ArrowDown":      0x7d,
	"ArrowLeft":      0x7b,
	"ArrowRight":     0x7c,
	"Escape":         0x35,
	"Tab":            0x30,
	"Space":          0x31,
	"CapsLock":       0x39,
	"ShiftLeft":      0x38,
	"ShiftRight":     0x3c,
	"ControlLeft":    0x3b,
	"ControlRight":   0x3e,
	"AltLeft":        0x3a,
	"AltRight":       0x3d,
	"MetaLeft":       0x37,
	"MetaRight":      0x36,
}
