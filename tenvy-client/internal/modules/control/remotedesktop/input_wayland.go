//go:build linux

package remotedesktop

/*
#cgo CFLAGS: -D_GNU_SOURCE
#include <errno.h>
#include <linux/input-event-codes.h>
#include <linux/input.h>
#include <linux/uinput.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>
#include <unistd.h>

static int ui_set_evbit(int fd, int evbit) { return ioctl(fd, UI_SET_EVBIT, evbit); }
static int ui_set_keybit(int fd, int keybit) { return ioctl(fd, UI_SET_KEYBIT, keybit); }
static int ui_set_relbit(int fd, int relbit) { return ioctl(fd, UI_SET_RELBIT, relbit); }
static int ui_set_absbit(int fd, int absbit) { return ioctl(fd, UI_SET_ABSBIT, absbit); }
static int ui_dev_create(int fd) { return ioctl(fd, UI_DEV_CREATE); }
static int ui_dev_destroy(int fd) { return ioctl(fd, UI_DEV_DESTROY); }
static int ui_write_device(int fd, const char *name) {
        struct uinput_user_dev uidev;
        memset(&uidev, 0, sizeof(uidev));
        snprintf(uidev.name, UINPUT_MAX_NAME_SIZE, "%s", name);
        uidev.id.bustype = BUS_USB;
        uidev.id.vendor = 0x1;
        uidev.id.product = 0x1;
        uidev.id.version = 1;
        uidev.absmin[ABS_X] = 0;
        uidev.absmax[ABS_X] = 65535;
        uidev.absmin[ABS_Y] = 0;
        uidev.absmax[ABS_Y] = 65535;
        if (write(fd, &uidev, sizeof(uidev)) < 0) {
                return -1;
        }
        return 0;
}
static int current_errno(void) { return errno; }
*/
import "C"

import (
	"errors"
	"fmt"
	"image"
	"io"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unicode"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	waylandPointerMax = 65535
	waylandDeviceName = "Tenvy Wayland Remote Input"
)

var (
	waylandOnce    sync.Once
	waylandBackend linuxInputBackend
	waylandInitErr error
)

func getWaylandBackend() (linuxInputBackend, error) {
	waylandOnce.Do(func() {
		backend, err := newWaylandInput()
		if err != nil {
			waylandInitErr = err
			return
		}
		waylandBackend = backend
	})
	return waylandBackend, waylandInitErr
}

type waylandInput struct {
	fd     int
	file   *os.File
	mu     sync.Mutex
	closed bool
}

func newWaylandInput() (linuxInputBackend, error) {
	devicePath := "/dev/uinput"
	file, err := os.OpenFile(devicePath, os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", devicePath, err)
	}

	input := &waylandInput{fd: int(file.Fd()), file: file}
	if err := input.initializeDevice(); err != nil {
		file.Close()
		return nil, err
	}

	runtime.SetFinalizer(input, (*waylandInput).finalize)
	return input, nil
}

func (w *waylandInput) finalize() {
	_ = w.close()
}

func (w *waylandInput) close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	if rc := C.ui_dev_destroy(C.int(w.fd)); rc != 0 {
		errno := unix.Errno(C.current_errno())
		return fmt.Errorf("failed to destroy uinput device: %w", errno)
	}
	return w.file.Close()
}

func (w *waylandInput) initializeDevice() error {
	fd := C.int(w.fd)
	if rc := C.ui_set_evbit(fd, C.EV_SYN); rc != 0 {
		errno := unix.Errno(C.current_errno())
		return fmt.Errorf("failed to enable EV_SYN: %w", errno)
	}
	if rc := C.ui_set_evbit(fd, C.EV_KEY); rc != 0 {
		errno := unix.Errno(C.current_errno())
		return fmt.Errorf("failed to enable EV_KEY: %w", errno)
	}
	if rc := C.ui_set_evbit(fd, C.EV_ABS); rc != 0 {
		errno := unix.Errno(C.current_errno())
		return fmt.Errorf("failed to enable EV_ABS: %w", errno)
	}
	if rc := C.ui_set_evbit(fd, C.EV_REL); rc != 0 {
		errno := unix.Errno(C.current_errno())
		return fmt.Errorf("failed to enable EV_REL: %w", errno)
	}

	for code := C.int(1); code <= C.KEY_MAX; code++ {
		if rc := C.ui_set_keybit(fd, code); rc != 0 {
			errno := unix.Errno(C.current_errno())
			return fmt.Errorf("failed to enable key %d: %w", int(code), errno)
		}
	}

	if rc := C.ui_set_absbit(fd, C.ABS_X); rc != 0 {
		errno := unix.Errno(C.current_errno())
		return fmt.Errorf("failed to enable ABS_X: %w", errno)
	}
	if rc := C.ui_set_absbit(fd, C.ABS_Y); rc != 0 {
		errno := unix.Errno(C.current_errno())
		return fmt.Errorf("failed to enable ABS_Y: %w", errno)
	}
	if rc := C.ui_set_relbit(fd, C.REL_WHEEL); rc != 0 {
		errno := unix.Errno(C.current_errno())
		return fmt.Errorf("failed to enable REL_WHEEL: %w", errno)
	}
	if rc := C.ui_set_relbit(fd, C.REL_HWHEEL); rc != 0 {
		errno := unix.Errno(C.current_errno())
		return fmt.Errorf("failed to enable REL_HWHEEL: %w", errno)
	}

	name := C.CString(waylandDeviceName)
	defer C.free(unsafe.Pointer(name))
	if rc := C.ui_write_device(fd, name); rc != 0 {
		errno := unix.Errno(C.current_errno())
		return fmt.Errorf("failed to configure uinput device: %w", errno)
	}
	if rc := C.ui_dev_create(fd); rc != 0 {
		errno := unix.Errno(C.current_errno())
		return fmt.Errorf("failed to create uinput device: %w", errno)
	}
	return nil
}

func (w *waylandInput) Process(monitors []remoteMonitor, settings RemoteDesktopSettings, events []RemoteDesktopInputEvent) error {
	fallback := selectMonitorForInputLinux(monitors, settings.Monitor, defaultWaylandMonitor(monitors))

	w.mu.Lock()
	defer w.mu.Unlock()

	for _, event := range events {
		switch event.Type {
		case RemoteInputMouseMove:
			target := monitorFromEventLinux(monitors, fallback, event.Monitor)
			if err := w.movePointer(event, target); err != nil {
				return err
			}
		case RemoteInputMouseButton:
			if err := w.sendMouseButton(event.Button, event.Pressed); err != nil {
				return err
			}
		case RemoteInputMouseScroll:
			if err := w.sendMouseScroll(event); err != nil {
				return err
			}
		case RemoteInputKey:
			if err := w.sendKeyEvent(event); err != nil {
				return err
			}
		}
	}
	return nil
}

func defaultWaylandMonitor(monitors []remoteMonitor) remoteMonitor {
	if len(monitors) > 0 {
		return monitors[0]
	}
	rect := image.Rect(0, 0, 1920, 1080)
	info := RemoteDesktopMonitorInfo{Width: rect.Dx(), Height: rect.Dy()}
	return remoteMonitor{info: info, bounds: rect}
}

func (w *waylandInput) movePointer(event RemoteDesktopInputEvent, monitor remoteMonitor) error {
	targetX, targetY := resolvePointerPosition(event, monitor)
	bounds := monitor.bounds
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 {
		width = maxInt(monitor.info.Width, 1)
		bounds.Max.X = bounds.Min.X + width
	}
	if height <= 0 {
		height = maxInt(monitor.info.Height, 1)
		bounds.Max.Y = bounds.Min.Y + height
	}

	denomX := math.Max(float64(width-1), 1)
	denomY := math.Max(float64(height-1), 1)
	normX := clampFloat((targetX-float64(bounds.Min.X))/denomX, 0, 1)
	normY := clampFloat((targetY-float64(bounds.Min.Y))/denomY, 0, 1)

	absX := int32(math.Round(normX * waylandPointerMax))
	absY := int32(math.Round(normY * waylandPointerMax))
	return w.sendAbsolute(absX, absY)
}

func (w *waylandInput) sendAbsolute(x, y int32) error {
	if err := w.sendEvent(uint16(C.EV_ABS), uint16(C.ABS_X), x); err != nil {
		return err
	}
	if err := w.sendEvent(uint16(C.EV_ABS), uint16(C.ABS_Y), y); err != nil {
		return err
	}
	return w.sync()
}

func (w *waylandInput) sendMouseButton(button RemoteDesktopMouseButton, pressed bool) error {
	code, ok := waylandButtonCodes[button]
	if !ok {
		return nil
	}
	value := int32(0)
	if pressed {
		value = 1
	}
	if err := w.sendEvent(uint16(C.EV_KEY), code, value); err != nil {
		return err
	}
	return w.sync()
}

func (w *waylandInput) sendMouseScroll(event RemoteDesktopInputEvent) error {
	vertical := scrollSteps(event.DeltaY, event.DeltaMode)
	horizontal := scrollSteps(event.DeltaX, event.DeltaMode)

	if vertical != 0 {
		if err := w.sendEvent(uint16(C.EV_REL), uint16(C.REL_WHEEL), int32(-vertical)); err != nil {
			return err
		}
	}
	if horizontal != 0 {
		if err := w.sendEvent(uint16(C.EV_REL), uint16(C.REL_HWHEEL), int32(horizontal)); err != nil {
			return err
		}
	}
	if vertical == 0 && horizontal == 0 {
		return nil
	}
	return w.sync()
}

func (w *waylandInput) sendKeyEvent(event RemoteDesktopInputEvent) error {
	code, ok := resolveEvdevCode(event)
	if !ok {
		return errors.New("unsupported key event")
	}
	value := int32(0)
	if event.Pressed {
		value = 1
		if event.Repeat {
			value = 2
		}
	}
	if err := w.sendEvent(uint16(C.EV_KEY), code, value); err != nil {
		return err
	}
	return w.sync()
}

func (w *waylandInput) sendEvent(eventType uint16, code uint16, value int32) error {
	ev := inputEvent{Type: eventType, Code: code, Value: value}
	buf := (*(*[unsafe.Sizeof(ev)]byte)(unsafe.Pointer(&ev)))[:]
	n, err := unix.Write(w.fd, buf)
	if err != nil {
		return err
	}
	if n != len(buf) {
		return io.ErrShortWrite
	}
	return nil
}

func (w *waylandInput) sync() error {
	return w.sendEvent(uint16(C.EV_SYN), uint16(C.SYN_REPORT), 0)
}

var waylandButtonCodes = map[RemoteDesktopMouseButton]uint16{
	RemoteMouseButtonLeft:   uint16(C.BTN_LEFT),
	RemoteMouseButtonMiddle: uint16(C.BTN_MIDDLE),
	RemoteMouseButtonRight:  uint16(C.BTN_RIGHT),
}

type inputEvent struct {
	Time  unix.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

func resolveEvdevCode(event RemoteDesktopInputEvent) (uint16, bool) {
	if code, ok := evdevCodeFromCode(event.Code); ok {
		return code, true
	}
	if code, ok := evdevCodeFromKeyName(event.Key); ok {
		return code, true
	}
	if code, ok := evdevKeyCodeMap[event.KeyCode]; ok {
		return code, true
	}
	if event.KeyCode >= 65 && event.KeyCode <= 90 { // letters A-Z
		return uint16(int(C.KEY_A) + event.KeyCode - 65), true
	}
	if event.KeyCode >= 48 && event.KeyCode <= 57 { // digits 0-9
		if event.KeyCode == 48 {
			return uint16(C.KEY_0), true
		}
		return uint16(int(C.KEY_1) + event.KeyCode - 49), true
	}
	return 0, false
}

func evdevCodeFromCode(code string) (uint16, bool) {
	if code == "" {
		return 0, false
	}
	if value, ok := evdevLookup[code]; ok {
		return value, true
	}
	if strings.HasPrefix(code, "Key") && len(code) == 4 {
		r := rune(code[3])
		if r >= 'A' && r <= 'Z' {
			return uint16(int(C.KEY_A) + int(r-'A')), true
		}
	}
	if strings.HasPrefix(code, "Digit") && len(code) == 6 {
		r := rune(code[5])
		if r >= '0' && r <= '9' {
			return digitKeyCode(r), true
		}
	}
	if strings.HasPrefix(code, "Numpad") {
		if value, ok := evdevLookup[code]; ok {
			return value, true
		}
	}
	if strings.HasPrefix(code, "F") {
		if value, ok := evdevLookup[code]; ok {
			return value, true
		}
	}
	return 0, false
}

func evdevCodeFromKeyName(name string) (uint16, bool) {
	if name == "" {
		return 0, false
	}
	if value, ok := evdevLookup[name]; ok {
		return value, true
	}
	if len(name) == 1 {
		r := []rune(name)[0]
		if unicode.IsLetter(r) {
			upper := unicode.ToUpper(r)
			return uint16(int(C.KEY_A) + int(upper-'A')), true
		}
		if unicode.IsDigit(r) {
			return digitKeyCode(r), true
		}
	}
	return 0, false
}

func digitKeyCode(r rune) uint16 {
	if r == '0' {
		return uint16(C.KEY_0)
	}
	return uint16(int(C.KEY_1) + int(r-'1'))
}

var evdevLookup = map[string]uint16{
	"Backspace":          uint16(C.KEY_BACKSPACE),
	"Tab":                uint16(C.KEY_TAB),
	"Enter":              uint16(C.KEY_ENTER),
	"Escape":             uint16(C.KEY_ESC),
	"Space":              uint16(C.KEY_SPACE),
	"Minus":              uint16(C.KEY_MINUS),
	"Equal":              uint16(C.KEY_EQUAL),
	"BracketLeft":        uint16(C.KEY_LEFTBRACE),
	"BracketRight":       uint16(C.KEY_RIGHTBRACE),
	"Backslash":          uint16(C.KEY_BACKSLASH),
	"Semicolon":          uint16(C.KEY_SEMICOLON),
	"Quote":              uint16(C.KEY_APOSTROPHE),
	"Backquote":          uint16(C.KEY_GRAVE),
	"Comma":              uint16(C.KEY_COMMA),
	"Period":             uint16(C.KEY_DOT),
	"Slash":              uint16(C.KEY_SLASH),
	"CapsLock":           uint16(C.KEY_CAPSLOCK),
	"ShiftLeft":          uint16(C.KEY_LEFTSHIFT),
	"ShiftRight":         uint16(C.KEY_RIGHTSHIFT),
	"ControlLeft":        uint16(C.KEY_LEFTCTRL),
	"ControlRight":       uint16(C.KEY_RIGHTCTRL),
	"AltLeft":            uint16(C.KEY_LEFTALT),
	"AltRight":           uint16(C.KEY_RIGHTALT),
	"MetaLeft":           uint16(C.KEY_LEFTMETA),
	"MetaRight":          uint16(C.KEY_RIGHTMETA),
	"ContextMenu":        uint16(C.KEY_MENU),
	"ArrowUp":            uint16(C.KEY_UP),
	"ArrowDown":          uint16(C.KEY_DOWN),
	"ArrowLeft":          uint16(C.KEY_LEFT),
	"ArrowRight":         uint16(C.KEY_RIGHT),
	"Delete":             uint16(C.KEY_DELETE),
	"Insert":             uint16(C.KEY_INSERT),
	"Home":               uint16(C.KEY_HOME),
	"End":                uint16(C.KEY_END),
	"PageUp":             uint16(C.KEY_PAGEUP),
	"PageDown":           uint16(C.KEY_PAGEDOWN),
	"PrintScreen":        uint16(C.KEY_SYSRQ),
	"ScrollLock":         uint16(C.KEY_SCROLLLOCK),
	"Pause":              uint16(C.KEY_PAUSE),
	"NumLock":            uint16(C.KEY_NUMLOCK),
	"IntlBackslash":      uint16(C.KEY_102ND),
	"NumpadDivide":       uint16(C.KEY_KPSLASH),
	"NumpadMultiply":     uint16(C.KEY_KPASTERISK),
	"NumpadSubtract":     uint16(C.KEY_KPMINUS),
	"NumpadAdd":          uint16(C.KEY_KPPLUS),
	"NumpadDecimal":      uint16(C.KEY_KPDOT),
	"NumpadComma":        uint16(C.KEY_KPCOMMA),
	"NumpadEnter":        uint16(C.KEY_KPENTER),
	"NumpadEqual":        uint16(C.KEY_KPEQUAL),
	"F1":                 uint16(C.KEY_F1),
	"F2":                 uint16(C.KEY_F2),
	"F3":                 uint16(C.KEY_F3),
	"F4":                 uint16(C.KEY_F4),
	"F5":                 uint16(C.KEY_F5),
	"F6":                 uint16(C.KEY_F6),
	"F7":                 uint16(C.KEY_F7),
	"F8":                 uint16(C.KEY_F8),
	"F9":                 uint16(C.KEY_F9),
	"F10":                uint16(C.KEY_F10),
	"F11":                uint16(C.KEY_F11),
	"F12":                uint16(C.KEY_F12),
	"F13":                uint16(C.KEY_F13),
	"F14":                uint16(C.KEY_F14),
	"F15":                uint16(C.KEY_F15),
	"F16":                uint16(C.KEY_F16),
	"F17":                uint16(C.KEY_F17),
	"F18":                uint16(C.KEY_F18),
	"F19":                uint16(C.KEY_F19),
	"F20":                uint16(C.KEY_F20),
	"F21":                uint16(C.KEY_F21),
	"F22":                uint16(C.KEY_F22),
	"F23":                uint16(C.KEY_F23),
	"F24":                uint16(C.KEY_F24),
	"AudioVolumeMute":    uint16(C.KEY_MUTE),
	"AudioVolumeUp":      uint16(C.KEY_VOLUMEUP),
	"AudioVolumeDown":    uint16(C.KEY_VOLUMEDOWN),
	"MediaPlayPause":     uint16(C.KEY_PLAYPAUSE),
	"MediaTrackNext":     uint16(C.KEY_NEXTSONG),
	"MediaTrackPrevious": uint16(C.KEY_PREVIOUSSONG),
	"MediaStop":          uint16(C.KEY_STOPCD),
	"BrowserBack":        uint16(C.KEY_BACK),
	"BrowserForward":     uint16(C.KEY_FORWARD),
	"BrowserRefresh":     uint16(C.KEY_REFRESH),
	"BrowserStop":        uint16(C.KEY_STOP),
	"BrowserHome":        uint16(C.KEY_HOMEPAGE),
	"LaunchMail":         uint16(C.KEY_MAIL),
	"Lang1":              uint16(C.KEY_HANGEUL),
	"Lang2":              uint16(C.KEY_HANJA),
	"KanaMode":           uint16(C.KEY_KATAKANA),
	"HangulMode":         uint16(C.KEY_HANGEUL),
	"HanjaMode":          uint16(C.KEY_HANJA),
	"Hiragana":           uint16(C.KEY_HIRAGANA),
	"Katakana":           uint16(C.KEY_KATAKANA),
	"KanjiMode":          uint16(C.KEY_KATAKANAHIRAGANA),
}

var evdevKeyCodeMap = map[int]uint16{
	8:   uint16(C.KEY_BACKSPACE),
	9:   uint16(C.KEY_TAB),
	13:  uint16(C.KEY_ENTER),
	16:  uint16(C.KEY_LEFTSHIFT),
	17:  uint16(C.KEY_LEFTCTRL),
	18:  uint16(C.KEY_LEFTALT),
	20:  uint16(C.KEY_CAPSLOCK),
	27:  uint16(C.KEY_ESC),
	32:  uint16(C.KEY_SPACE),
	33:  uint16(C.KEY_PAGEUP),
	34:  uint16(C.KEY_PAGEDOWN),
	35:  uint16(C.KEY_END),
	36:  uint16(C.KEY_HOME),
	37:  uint16(C.KEY_LEFT),
	38:  uint16(C.KEY_UP),
	39:  uint16(C.KEY_RIGHT),
	40:  uint16(C.KEY_DOWN),
	45:  uint16(C.KEY_INSERT),
	46:  uint16(C.KEY_DELETE),
	91:  uint16(C.KEY_LEFTMETA),
	92:  uint16(C.KEY_RIGHTMETA),
	93:  uint16(C.KEY_MENU),
	96:  uint16(C.KEY_KP0),
	97:  uint16(C.KEY_KP1),
	98:  uint16(C.KEY_KP2),
	99:  uint16(C.KEY_KP3),
	100: uint16(C.KEY_KP4),
	101: uint16(C.KEY_KP5),
	102: uint16(C.KEY_KP6),
	103: uint16(C.KEY_KP7),
	104: uint16(C.KEY_KP8),
	105: uint16(C.KEY_KP9),
	106: uint16(C.KEY_KPASTERISK),
	107: uint16(C.KEY_KPPLUS),
	109: uint16(C.KEY_KPMINUS),
	110: uint16(C.KEY_KPDOT),
	111: uint16(C.KEY_KPSLASH),
	112: uint16(C.KEY_F1),
	113: uint16(C.KEY_F2),
	114: uint16(C.KEY_F3),
	115: uint16(C.KEY_F4),
	116: uint16(C.KEY_F5),
	117: uint16(C.KEY_F6),
	118: uint16(C.KEY_F7),
	119: uint16(C.KEY_F8),
	120: uint16(C.KEY_F9),
	121: uint16(C.KEY_F10),
	122: uint16(C.KEY_F11),
	123: uint16(C.KEY_F12),
	144: uint16(C.KEY_NUMLOCK),
	145: uint16(C.KEY_SCROLLLOCK),
	173: uint16(C.KEY_MUTE),
	174: uint16(C.KEY_VOLUMEDOWN),
	175: uint16(C.KEY_VOLUMEUP),
	176: uint16(C.KEY_NEXTSONG),
	177: uint16(C.KEY_PREVIOUSSONG),
	178: uint16(C.KEY_STOPCD),
	179: uint16(C.KEY_PLAYPAUSE),
	186: uint16(C.KEY_SEMICOLON),
	187: uint16(C.KEY_EQUAL),
	188: uint16(C.KEY_COMMA),
	189: uint16(C.KEY_MINUS),
	190: uint16(C.KEY_DOT),
	191: uint16(C.KEY_SLASH),
	192: uint16(C.KEY_GRAVE),
	219: uint16(C.KEY_LEFTBRACE),
	220: uint16(C.KEY_BACKSLASH),
	221: uint16(C.KEY_RIGHTBRACE),
	222: uint16(C.KEY_APOSTROPHE),
}
