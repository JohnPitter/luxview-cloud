//go:build windows

package main

// Display framing for the non-exclusive modes. The Serious Engine only knows two
// states in its settings (m_bActiveFullScreen 0/1); the launcher adds a third by
// running the engine WINDOWED and reshaping its render window:
//
//   - "windowed"   → a real, draggable titled window centered on screen, client
//                    snapped to the backbuffer size (no black gap).
//   - "borderless" → "janela em tela cheia": a titled window positioned at the
//                    top-left and sized to FILL the whole screen. The engine renders
//                    the backbuffer 1:1 at the top-left of the client (it does NOT
//                    stretch), so the backbuffer must equal the full-screen client —
//                    that resolution is forced on save (see fillScreenResolution).
//                    Keeping the Windows title bar is intentional (the player asked
//                    for "fullscreen but with the window border, like windowed").
//
// In BOTH modes we (a) activate the game window once it's framed so the engine
// resumes its render/input loop instead of staying paused in the background, and
// (b) run keepWindowAccessible for the whole session so the game never stays
// always-on-top (which would hide the launcher and the Alt+Tab switcher).
//
// Switching OUT of the game: the engine grabs the keyboard exclusively (DirectInput)
// and GameGuard blocks task-switching while the game is focused, so raw Alt+Tab is
// (a) often blocked and (b) when it does fire, it forces a DX9 device loss/reset that
// this 2007 engine sometimes crashes on. The safe path is the global Ctrl+Alt+M
// hotkey (runMinimizeHotkey): a clean minimize from a separate process, no focus-loss
// mid-render, nothing for the anti-cheat to flag. We deliberately keep our hands off
// the window during focus transitions so we don't add to that fragility.
//
// The render window is matched by process (rakion.bin) + window class "Rakion",
// never the "Loading..." splash or other apps' windows. Note: third-party overlays
// (Discord/NVIDIA) only render correctly over this game in EXCLUSIVE FULLSCREEN; in
// any windowed mode they paint a black top-most layer — an overlay limitation, which
// is why exclusive fullscreen stays the default.

import (
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procGetClientRect            = user32.NewProc("GetClientRect")
	procGetClassName             = user32.NewProc("GetClassNameW")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procGetWindowLongPtr         = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtr         = user32.NewProc("SetWindowLongPtrW")
	procAdjustWindowRect         = user32.NewProc("AdjustWindowRect")
	procSetWindowText            = user32.NewProc("SetWindowTextW")
	procGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
	procSetThreadDpiAwarenessCtx = user32.NewProc("SetThreadDpiAwarenessContext")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procBringWindowToTop         = user32.NewProc("BringWindowToTop")
	procRegisterHotKey           = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey         = user32.NewProc("UnregisterHotKey")
	procPeekMessage              = user32.NewProc("PeekMessageW")
	procShowWindow               = user32.NewProc("ShowWindow")
	procIsIconic                 = user32.NewProc("IsIconic")
)

const (
	gwlStyle      = ^uintptr(15) // GWL_STYLE = -16
	gwlExStyle    = ^uintptr(19) // GWL_EXSTYLE = -20
	wsPopup       = 0x80000000
	wsCaption     = 0x00C00000
	wsSysMenu     = 0x00080000
	wsMinimizeBox = 0x00020000
	wsExTopmost   = 0x00000008

	swpNoSize      = 0x0001
	swpNoMove      = 0x0002
	swpNoZOrder    = 0x0004
	swpNoActivate  = 0x0010
	swpFrameChange = 0x0020
	swpShowWindow  = 0x0040
	hwndNotopmost  = ^uintptr(1) // HWND_NOTOPMOST = -2

	smCXScreen    = 0
	smCYScreen    = 1
	dpiCtxUnaware = ^uintptr(0) // DPI_AWARENESS_CONTEXT_UNAWARE = (HANDLE)-1
	titledStyle   = wsCaption | wsSysMenu | wsMinimizeBox
	gameWndClass  = "Rakion" // the Serious Engine render window class

	// Global "minimize the game" hotkey (Ctrl+Alt+M). The game grabs the keyboard
	// exclusively while focused (DirectInput) and GameGuard blocks Alt+Tab; since the
	// launcher is a separate process, a RegisterHotKey hotkey still reaches us and
	// lets us yank the game out of the way — without touching its memory.
	modAlt        = 0x0001
	modControl    = 0x0002
	modNoRepeat   = 0x4000
	vkMinimizeKey = 0x4D     // 'M'
	hotkeyID      = 1        // arbitrary, unique within this thread
	wmHotkey      = 0x0312   // WM_HOTKEY
	pmRemove      = 0x0001   // PM_REMOVE
	swMinimize    = 6        // SW_MINIMIZE
	swRestore     = 9        // SW_RESTORE
)

// msg mirrors the Win32 MSG struct for PeekMessage.
type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

type winRect struct{ Left, Top, Right, Bottom int32 }

func classNameOf(hwnd uintptr) string {
	buf := make([]uint16, 64)
	procGetClassName.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf)
}

func clientSize(hwnd uintptr) (int32, int32) {
	var rc winRect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	return rc.Right - rc.Left, rc.Bottom - rc.Top
}

func screenSize() (int32, int32) {
	w, _, _ := procGetSystemMetrics.Call(smCXScreen)
	h, _, _ := procGetSystemMetrics.Call(smCYScreen)
	return int32(w), int32(h)
}

// withDPIUnaware runs fn on a thread pinned to the game's DPI context so screen
// metrics and window coordinates line up with the (DPI-unaware) game.
func withDPIUnaware(fn func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if procSetThreadDpiAwarenessCtx.Find() == nil {
		if prev, _, _ := procSetThreadDpiAwarenessCtx.Call(dpiCtxUnaware); prev != 0 {
			defer procSetThreadDpiAwarenessCtx.Call(prev)
		}
	}
	fn()
}

// fillScreenResolution returns the client size of a titled window that fills the
// whole screen — i.e. the backbuffer the engine must use so "janela em tela cheia"
// has no black margins. Computed in the game's DPI-unaware space.
func fillScreenResolution() (int, int) {
	var cw, ch int
	withDPIUnaware(func() {
		scrW, scrH := screenSize()
		// Chrome (title bar + borders) for a titled window, via AdjustWindowRect on a
		// probe rect; the difference is the non-client size to subtract from the screen.
		probe := winRect{0, 0, 1000, 1000}
		procAdjustWindowRect.Call(uintptr(unsafe.Pointer(&probe)), titledStyle, 0)
		chromeW := (probe.Right - probe.Left) - 1000
		chromeH := (probe.Bottom - probe.Top) - 1000
		cw = int(scrW - chromeW)
		ch = int(scrH - chromeH)
	})
	if cw < 320 || ch < 240 {
		return 0, 0
	}
	return cw, ch
}

// focusWindow makes hwnd the foreground/active window so the engine's render loop
// resumes (it pauses while the window is in the background) — the launcher process
// owns the foreground at launch time, so it's allowed to hand it to the game.
func focusWindow(hwnd uintptr) {
	procBringWindowToTop.Call(hwnd)
	procSetForegroundWindow.Call(hwnd)
}

// clearTopmost drops the WS_EX_TOPMOST bit (and re-bands the window) so Alt+Tab can
// raise other windows over the game.
func clearTopmost(hwnd uintptr) {
	if ex, _, _ := procGetWindowLongPtr.Call(hwnd, gwlExStyle); ex&wsExTopmost != 0 {
		procSetWindowPos.Call(hwnd, hwndNotopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
	}
}

// gwEnumCb finds the game's render window: visible, owned by the game process and
// of the engine's window class (not the "Loading..." splash).
var (
	gwWantPID uint32
	gwFound   uintptr
)

var gwEnumCb = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
	var wpid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&wpid)))
	if wpid != gwWantPID {
		return 1
	}
	if r, _, _ := procIsWindowVisible.Call(hwnd); r == 0 {
		return 1
	}
	if classNameOf(hwnd) != gameWndClass {
		return 1
	}
	gwFound = hwnd
	return 0
})

func findGameWindow(pid uint32) uintptr {
	gwWantPID = pid
	gwFound = 0
	procEnumWindows.Call(gwEnumCb, 0)
	return gwFound
}

// waitForGameWindow polls until the game's render window exists, up to ~tries*500ms.
func waitForGameWindow(processName string, tries int) uintptr {
	for range tries {
		if pid := gameProcessPID(processName); pid != 0 {
			if hwnd := findGameWindow(pid); hwnd != 0 {
				return hwnd
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return 0
}

// frameGameWindow waits for the game's render window and reshapes it for the given
// display mode ("windowed" or "borderless"/fill-screen), re-applying while the engine
// re-pins/restyles it during init. No-op for fullscreen (exclusive needs no framing).
func frameGameWindow(processName, mode string) {
	if processName == "" || mode == displayFullscreen {
		return
	}
	fill := mode == displayBorderless

	// Keep the window Alt+Tab-able for the whole session (independent of framing).
	go keepWindowAccessible(processName)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if procSetThreadDpiAwarenessCtx.Find() == nil {
		if prev, _, _ := procSetThreadDpiAwarenessCtx.Call(dpiCtxUnaware); prev != 0 {
			defer procSetThreadDpiAwarenessCtx.Call(prev)
		}
	}

	hwnd := waitForGameWindow(processName, 180) // ~90s — MD5 checks run before the window
	if hwnd == 0 {
		return
	}
	// The render window starts at 1x1; wait for its final client size.
	var cw, ch int32
	for range 60 {
		if cw, ch = clientSize(hwnd); cw >= 320 && ch >= 240 {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}

	applyTitledFrame(hwnd, cw, ch, true, fill) // activate the game window on the first apply
	for range 24 {                             // re-apply while the engine re-pins/restyles during init
		time.Sleep(500 * time.Millisecond)
		style, _, _ := procGetWindowLongPtr.Call(hwnd, gwlStyle)
		var rc winRect
		procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		lostCaption := style&wsCaption == 0
		if fill {
			// Fill mode wants the window pinned at the top-left; re-apply if it lost
			// the title bar or the engine moved it away from the corner.
			if lostCaption || rc.Left > 40 || rc.Top > 40 {
				applyTitledFrame(hwnd, cw, ch, false, true)
			}
		} else if lostCaption || (rc.Left < 40 && rc.Top < 40) {
			// Windowed wants it centered; re-apply if it lost the title bar or the
			// engine re-pinned it back to the corner.
			applyTitledFrame(hwnd, cw, ch, false, false)
		}
	}
}

// applyTitledFrame gives the borderless render window a title bar and either centers
// it (windowed) or pins it to the top-left filling the screen (fill). The client is
// snapped to clientW x clientH so the engine's backbuffer fills it exactly (no gap).
func applyTitledFrame(hwnd uintptr, clientW, clientH int32, activate, fill bool) {
	style, _, _ := procGetWindowLongPtr.Call(hwnd, gwlStyle)
	style &^= wsPopup
	style |= titledStyle
	procSetWindowLongPtr.Call(hwnd, gwlStyle, style)
	clearTopmost(hwnd)

	title, _ := windows.UTF16PtrFromString("Rakion — LuxView Cloud")
	procSetWindowText.Call(hwnd, uintptr(unsafe.Pointer(title)))

	// Estimate the window size for a clientW x clientH client with this frame.
	rc := winRect{0, 0, clientW, clientH}
	procAdjustWindowRect.Call(uintptr(unsafe.Pointer(&rc)), style, 0)
	winW := rc.Right - rc.Left
	winH := rc.Bottom - rc.Top
	placeWindow(hwnd, winW, winH, activate, fill)

	// AdjustWindowRect doesn't know the real DWM/theme border, so the actual client
	// can be a couple px off (a black gap in the corner). Measure and correct.
	if acw, ach := clientSize(hwnd); acw > 0 && ach > 0 && (acw != clientW || ach != clientH) {
		winW += clientW - acw
		winH += clientH - ach
		placeWindow(hwnd, winW, winH, activate, fill)
	}
	if activate {
		focusWindow(hwnd)
	}
}

// placeWindow sizes the window to winW x winH and positions it: top-left for fill,
// centered otherwise.
func placeWindow(hwnd uintptr, winW, winH int32, activate, fill bool) {
	var x, y int32
	if !fill {
		scrW, scrH := screenSize()
		x = max((scrW-winW)/2, 0)
		y = max((scrH-winH)/2, 0)
	}
	flags := uintptr(swpFrameChange | swpNoZOrder | swpShowWindow)
	if !activate {
		flags |= swpNoActivate
	}
	procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(winW), uintptr(winH), flags)
}

// keepWindowAccessible runs for the whole game session and only keeps the game from
// staying always-on-top (so the launcher and the Alt+Tab switcher can come over it).
// The actual Alt+Tab lock is removed by patchKeyHook (see keyhook_windows.go).
func keepWindowAccessible(processName string) {
	hwnd := waitForGameWindow(processName, 240)
	if hwnd == 0 {
		return
	}
	for gameProcessPID(processName) != 0 {
		clearTopmost(hwnd) // no-op unless the engine pinned it topmost
		time.Sleep(500 * time.Millisecond)
	}
}

// runMinimizeHotkey registers a global Ctrl+Alt+M hotkey for the whole game session
// and toggles the game window minimized/restored when pressed. This is the safe way
// around the "can't Alt+Tab while the game is focused" limitation (the engine grabs
// the keyboard exclusively and GameGuard blocks task-switching): the launcher is a
// separate process, so the hotkey reaches us and we just minimize the game — no
// memory tampering, nothing for the anti-cheat to flag.
func runMinimizeHotkey(processName string) {
	if processName == "" {
		return
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if procSetThreadDpiAwarenessCtx.Find() == nil {
		if prev, _, _ := procSetThreadDpiAwarenessCtx.Call(dpiCtxUnaware); prev != 0 {
			defer procSetThreadDpiAwarenessCtx.Call(prev)
		}
	}
	if waitForGameWindow(processName, 240) == 0 {
		return
	}
	if r, _, _ := procRegisterHotKey.Call(0, hotkeyID, modControl|modAlt|modNoRepeat, vkMinimizeKey); r == 0 {
		return // already registered (another instance) or blocked
	}
	defer procUnregisterHotKey.Call(0, hotkeyID)

	var m msg
	for gameProcessPID(processName) != 0 {
		for {
			got, _, _ := procPeekMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, pmRemove)
			if got == 0 {
				break
			}
			if m.message == wmHotkey {
				toggleGameMinimized(processName)
			}
		}
		time.Sleep(60 * time.Millisecond)
	}
}

// toggleGameMinimized minimizes the game window if it's showing, or restores+focuses
// it if it's minimized.
func toggleGameMinimized(processName string) {
	pid := gameProcessPID(processName)
	if pid == 0 {
		return
	}
	hwnd := findGameWindow(pid)
	if hwnd == 0 {
		return
	}
	if r, _, _ := procIsIconic.Call(hwnd); r != 0 {
		procShowWindow.Call(hwnd, swRestore)
		focusWindow(hwnd)
	} else {
		procShowWindow.Call(hwnd, swMinimize)
	}
}
