//go:build linux

package remotedesktop

import (
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"strings"
	"sync"
	"unicode"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
)

type x11Input struct {
	conn       *xgb.Conn
	root       xproto.Window
	screenRect image.Rectangle
	minKeycode xproto.Keycode
	maxKeycode xproto.Keycode

	keymapOnce sync.Once
	keymapErr  error
	keycodes   map[xproto.Keysym]xproto.Keycode
	mu         sync.Mutex
}

var (
	x11InputOnce sync.Once
	x11Instance  *x11Input
	x11InitErr   error
)

const (
	x11MinCoordinate = -32768
	x11MaxCoordinate = 32767
)

func processRemoteInput(monitors []remoteMonitor, settings RemoteDesktopSettings, events []RemoteDesktopInputEvent) error {
	if len(events) == 0 {
		return nil
	}

	input, err := getX11Input()
	if err != nil {
		return err
	}

	fallback := selectMonitorForInputLinux(monitors, settings.Monitor, input.defaultMonitor())

	input.mu.Lock()
	defer input.mu.Unlock()

	for _, event := range events {
		switch event.Type {
		case RemoteInputMouseMove:
			target := monitorFromEventLinux(monitors, fallback, event.Monitor)
			if err := input.movePointer(event, target); err != nil {
				return err
			}
		case RemoteInputMouseButton:
			if err := input.sendMouseButton(event.Button, event.Pressed); err != nil {
				return err
			}
		case RemoteInputMouseScroll:
			if err := input.sendMouseScroll(event); err != nil {
				return err
			}
		case RemoteInputKey:
			if err := input.sendKeyEvent(event); err != nil {
				return err
			}
		}
	}

	input.conn.Sync()
	return nil
}

func getX11Input() (*x11Input, error) {
	x11InputOnce.Do(func() {
		inst, err := newX11Input()
		if err != nil {
			x11InitErr = err
			return
		}
		x11Instance = inst
	})
	return x11Instance, x11InitErr
}

func newX11Input() (*x11Input, error) {
	display := os.Getenv("DISPLAY")
	if display == "" {
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			return nil, errors.New("wayland session detected but X11 DISPLAY is not available")
		}
		return nil, errors.New("X11 DISPLAY environment variable is not set")
	}

	conn, err := xgb.NewConnDisplay(display)
	if err != nil {
		return nil, err
	}
	if err := xtest.Init(conn); err != nil {
		conn.Close()
		return nil, err
	}

	setup := xproto.Setup(conn)
	if setup == nil {
		conn.Close()
		return nil, errors.New("failed to query X11 setup")
	}
	screen := setup.DefaultScreen(conn)
	if screen == nil {
		conn.Close()
		return nil, errors.New("failed to resolve X11 default screen")
	}
	rect := image.Rect(0, 0, int(screen.WidthInPixels), int(screen.HeightInPixels))

	return &x11Input{
		conn:       conn,
		root:       screen.Root,
		screenRect: rect,
		minKeycode: setup.MinKeycode,
		maxKeycode: setup.MaxKeycode,
		keycodes:   make(map[xproto.Keysym]xproto.Keycode),
	}, nil
}

func (x *x11Input) defaultMonitor() remoteMonitor {
	info := RemoteDesktopMonitorInfo{Width: x.screenRect.Dx(), Height: x.screenRect.Dy()}
	return remoteMonitor{info: info, bounds: x.screenRect}
}

func selectMonitorForInputLinux(monitors []remoteMonitor, index int, fallback remoteMonitor) remoteMonitor {
	if len(monitors) == 0 {
		return fallback
	}
	if index < 0 || index >= len(monitors) {
		index = 0
	}
	return monitors[index]
}

func monitorFromEventLinux(monitors []remoteMonitor, fallback remoteMonitor, override *int) remoteMonitor {
	if override != nil {
		idx := *override
		if idx >= 0 && idx < len(monitors) {
			return monitors[idx]
		}
	}
	return fallback
}

func (x *x11Input) movePointer(event RemoteDesktopInputEvent, monitor remoteMonitor) error {
	targetX, targetY := resolvePointerPosition(event, monitor)
	point := image.Point{X: clampToInt(targetX), Y: clampToInt(targetY)}
	return xproto.WarpPointerChecked(x.conn, xproto.WindowNone, x.root, 0, 0, 0, 0, int16(point.X), int16(point.Y)).Check()
}

func (x *x11Input) sendMouseButton(button RemoteDesktopMouseButton, pressed bool) error {
	var detail byte
	switch button {
	case RemoteMouseButtonLeft:
		detail = 1
	case RemoteMouseButtonMiddle:
		detail = 2
	case RemoteMouseButtonRight:
		detail = 3
	default:
		return nil
	}
	eventType := byte(xproto.ButtonPress)
	if !pressed {
		eventType = byte(xproto.ButtonRelease)
	}
	return xtest.FakeInputChecked(x.conn, eventType, detail, 0, xproto.WindowNone, 0, 0, 0).Check()
}

func (x *x11Input) sendMouseScroll(event RemoteDesktopInputEvent) error {
	vertical := scrollSteps(event.DeltaY, event.DeltaMode)
	horizontal := scrollSteps(event.DeltaX, event.DeltaMode)

	for vertical > 0 {
		if err := x.clickScrollButton(5); err != nil {
			return err
		}
		vertical--
	}
	for vertical < 0 {
		if err := x.clickScrollButton(4); err != nil {
			return err
		}
		vertical++
	}
	for horizontal > 0 {
		if err := x.clickScrollButton(7); err != nil {
			return err
		}
		horizontal--
	}
	for horizontal < 0 {
		if err := x.clickScrollButton(6); err != nil {
			return err
		}
		horizontal++
	}
	return nil
}

func (x *x11Input) clickScrollButton(detail byte) error {
	if err := xtest.FakeInputChecked(x.conn, byte(xproto.ButtonPress), detail, 0, xproto.WindowNone, 0, 0, 0).Check(); err != nil {
		return err
	}
	return xtest.FakeInputChecked(x.conn, byte(xproto.ButtonRelease), detail, 0, xproto.WindowNone, 0, 0, 0).Check()
}

func scrollSteps(delta float64, mode int) int {
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
	amount := int(math.Round(delta * scale))
	if amount == 0 {
		if delta > 0 {
			amount = 1
		} else {
			amount = -1
		}
	}
	return amount
}

func (x *x11Input) sendKeyEvent(event RemoteDesktopInputEvent) error {
	keysym, ok := resolveX11Keysym(event)
	if !ok {
		return errors.New("unsupported key event")
	}
	keycode, err := x.keycodeForKeysym(keysym)
	if err != nil {
		return err
	}
	eventType := byte(xproto.KeyPress)
	if !event.Pressed {
		eventType = byte(xproto.KeyRelease)
	}
	return xtest.FakeInputChecked(x.conn, eventType, byte(keycode), 0, xproto.WindowNone, 0, 0, 0).Check()
}

func (x *x11Input) keycodeForKeysym(sym xproto.Keysym) (xproto.Keycode, error) {
	if sym == 0 {
		return 0, errors.New("invalid keysym")
	}
	x.keymapOnce.Do(func() {
		x.keymapErr = x.populateKeymap()
	})
	if x.keymapErr != nil {
		return 0, x.keymapErr
	}
	if code, ok := x.keycodes[sym]; ok {
		return code, nil
	}
	if sym >= 'A' && sym <= 'Z' {
		if code, ok := x.keycodes[sym+32]; ok {
			return code, nil
		}
	}
	if sym >= 'a' && sym <= 'z' {
		if code, ok := x.keycodes[sym-32]; ok {
			return code, nil
		}
	}
	return 0, errors.New("keysym not mapped")
}

func (x *x11Input) populateKeymap() error {
	count := int(x.maxKeycode) - int(x.minKeycode) + 1
	cookie := xproto.GetKeyboardMapping(x.conn, x.minKeycode, byte(count))
	reply, err := cookie.Reply()
	if err != nil {
		return err
	}
	perKeycode := int(reply.KeysymsPerKeycode)
	if perKeycode <= 0 {
		return errors.New("invalid X11 keyboard mapping")
	}
	for i := 0; i < count; i++ {
		keycode := xproto.Keycode(int(x.minKeycode) + i)
		for j := 0; j < perKeycode; j++ {
			sym := reply.Keysyms[i*perKeycode+j]
			if sym == 0 {
				continue
			}
			if _, exists := x.keycodes[sym]; !exists {
				x.keycodes[sym] = keycode
			}
		}
	}
	return nil
}

func clampToInt(value float64) int {
	rounded := int(math.Round(value))
	if rounded < x11MinCoordinate {
		return x11MinCoordinate
	}
	if rounded > x11MaxCoordinate {
		return x11MaxCoordinate
	}
	return rounded
}

func resolveX11Keysym(event RemoteDesktopInputEvent) (xproto.Keysym, bool) {
	if sym, ok := keysymForCode(event.Code); ok {
		return sym, true
	}
	if sym, ok := keysymForKey(event.Key); ok {
		return sym, true
	}
	if len(event.Key) == 1 {
		r := []rune(event.Key)[0]
		if !event.ShiftKey {
			r = unicode.ToLower(r)
		}
		return xproto.Keysym(r), true
	}
	if event.KeyCode > 0 && event.KeyCode < 65535 {
		return xproto.Keysym(event.KeyCode), true
	}
	return 0, false
}

func keysymForCode(code string) (xproto.Keysym, bool) {
	if code == "" {
		return 0, false
	}
	if sym, ok := x11KeycodeMap[code]; ok {
		return sym, true
	}
	if strings.HasPrefix(code, "Key") && len(code) == 4 {
		r := rune(code[3])
		return xproto.Keysym(unicode.ToLower(r)), true
	}
	if strings.HasPrefix(code, "Digit") && len(code) == 6 {
		r := rune(code[5])
		if r >= '0' && r <= '9' {
			return xproto.Keysym(r), true
		}
	}
	if strings.HasPrefix(code, "Numpad") && len(code) > 6 {
		if sym, ok := x11KeycodeMap[code]; ok {
			return sym, true
		}
	}
	return 0, false
}

func keysymForKey(key string) (xproto.Keysym, bool) {
	if key == "" {
		return 0, false
	}
	if sym, ok := x11KeynameMap[key]; ok {
		return sym, true
	}
	if len(key) == 1 {
		r := []rune(key)[0]
		return xproto.Keysym(r), true
	}
	return 0, false
}

var x11KeycodeMap = map[string]xproto.Keysym{
	"Backspace":      0xff08,
	"Tab":            0xff09,
	"Enter":          0xff0d,
	"Escape":         0xff1b,
	"Space":          0x0020,
	"ArrowUp":        0xff52,
	"ArrowDown":      0xff54,
	"ArrowLeft":      0xff51,
	"ArrowRight":     0xff53,
	"Delete":         0xffff,
	"Home":           0xff50,
	"End":            0xff57,
	"PageUp":         0xff55,
	"PageDown":       0xff56,
	"Insert":         0xff63,
	"CapsLock":       0xffe5,
	"ShiftLeft":      0xffe1,
	"ShiftRight":     0xffe2,
	"ControlLeft":    0xffe3,
	"ControlRight":   0xffe4,
	"AltLeft":        0xffe9,
	"AltRight":       0xffea,
	"MetaLeft":       0xffe7,
	"MetaRight":      0xffe8,
	"ContextMenu":    0xff67,
	"PrintScreen":    0xff61,
	"ScrollLock":     0xff14,
	"Pause":          0xff13,
	"NumpadMultiply": 0xffaa,
	"NumpadAdd":      0xffab,
	"NumpadSubtract": 0xffad,
	"NumpadDecimal":  0xffae,
	"NumpadDivide":   0xffaf,
	"NumpadEnter":    0xff8d,
}

var x11KeynameMap = map[string]xproto.Keysym{
	"Backspace":   0xff08,
	"Tab":         0xff09,
	"Enter":       0xff0d,
	"Escape":      0xff1b,
	" ":           0x0020,
	"ArrowUp":     0xff52,
	"ArrowDown":   0xff54,
	"ArrowLeft":   0xff51,
	"ArrowRight":  0xff53,
	"Delete":      0xffff,
	"Home":        0xff50,
	"End":         0xff57,
	"PageUp":      0xff55,
	"PageDown":    0xff56,
	"Insert":      0xff63,
	"CapsLock":    0xffe5,
	"Shift":       0xffe1,
	"Control":     0xffe3,
	"Alt":         0xffe9,
	"Meta":        0xffe7,
	"ContextMenu": 0xff67,
	"PrintScreen": 0xff61,
	"ScrollLock":  0xff14,
	"Pause":       0xff13,
}

func init() {
	for i := 1; i <= 12; i++ {
		sym := xproto.Keysym(0xffbd + i)
		code := fmt.Sprintf("F%d", i)
		x11KeycodeMap[code] = sym
		x11KeynameMap[code] = sym
	}
}
