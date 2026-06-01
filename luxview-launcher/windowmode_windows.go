//go:build windows

package main

// Windowed mode: give the game's render window a real title bar (so it can be
// dragged) and center it on the primary screen. We keep the CLIENT area equal to
// the engine's backbuffer size — AdjustWindowRect grows the window to fit the
// title bar/border around the unchanged client, and we then measure the real
// client and correct for the DWM/theme border so the backbuffer fills the client
// exactly (no black gap). The render window is matched by process (rakion.bin) +
// window class "Rakion", never the "Loading..." splash or other apps' windows.
//
// Note: third-party overlays (Discord/NVIDIA) only render correctly over this game
// in EXCLUSIVE FULLSCREEN; in windowed they paint a black top-most layer. That's
// an overlay limitation outside the launcher's control — fullscreen is the default
// for that reason; windowed is for players who don't use overlays.

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
)

const (
	gwlStyle      = ^uintptr(15) // GWL_STYLE = -16
	wsPopup       = 0x80000000
	wsCaption     = 0x00C00000
	wsSysMenu     = 0x00080000
	wsMinimizeBox = 0x00020000

	swpNoZOrder    = 0x0004
	swpNoActivate  = 0x0010
	swpFrameChange = 0x0020
	swpShowWindow  = 0x0040

	smCXScreen    = 0
	smCYScreen    = 1
	dpiCtxUnaware = ^uintptr(0) // DPI_AWARENESS_CONTEXT_UNAWARE = (HANDLE)-1
	gameWndClass  = "Rakion"    // the Serious Engine render window class
)

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

// frameGameWindow waits for the game's render window, gives it a centered title
// bar and keeps re-applying while the engine re-pins/restyles it during init.
// No-op for fullscreen.
func frameGameWindow(processName string) {
	if processName == "" {
		return
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Match the game's DPI context so coordinates line up on scaled displays.
	if procSetThreadDpiAwarenessCtx.Find() == nil {
		if prev, _, _ := procSetThreadDpiAwarenessCtx.Call(dpiCtxUnaware); prev != 0 {
			defer procSetThreadDpiAwarenessCtx.Call(prev)
		}
	}

	var hwnd uintptr
	for range 180 { // ~90s — the driver runs MD5 checks before the window appears
		if pid := gameProcessPID(processName); pid != 0 {
			if hwnd = findGameWindow(pid); hwnd != 0 {
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
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
	applyTitledFrame(hwnd, cw, ch)
	for range 24 { // re-apply while the engine re-pins/restyles during init
		time.Sleep(500 * time.Millisecond)
		style, _, _ := procGetWindowLongPtr.Call(hwnd, gwlStyle)
		var rc winRect
		procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		if style&wsCaption == 0 || (rc.Left < 40 && rc.Top < 40) {
			applyTitledFrame(hwnd, cw, ch)
		}
	}
}

// applyTitledFrame turns the borderless render window into a titled (draggable)
// window centered on screen, with the client area snapped to clientW x clientH so
// the engine's backbuffer fills it exactly (no black gap).
func applyTitledFrame(hwnd uintptr, clientW, clientH int32) {
	style, _, _ := procGetWindowLongPtr.Call(hwnd, gwlStyle)
	style &^= wsPopup
	style |= wsCaption | wsSysMenu | wsMinimizeBox
	procSetWindowLongPtr.Call(hwnd, gwlStyle, style)

	title, _ := windows.UTF16PtrFromString("Rakion — LuxView Cloud")
	procSetWindowText.Call(hwnd, uintptr(unsafe.Pointer(title)))

	// Estimate the window size for a clientW x clientH client with this frame.
	rc := winRect{0, 0, clientW, clientH}
	procAdjustWindowRect.Call(uintptr(unsafe.Pointer(&rc)), style, 0)
	winW := rc.Right - rc.Left
	winH := rc.Bottom - rc.Top
	placeCentered(hwnd, winW, winH)

	// AdjustWindowRect doesn't know the real DWM/theme border, so the actual client
	// can be a couple px off (a black gap in the corner). Measure and correct.
	if acw, ach := clientSize(hwnd); acw > 0 && ach > 0 && (acw != clientW || ach != clientH) {
		winW += clientW - acw
		winH += clientH - ach
		placeCentered(hwnd, winW, winH)
	}
}

// placeCentered sizes the window to winW x winH and centers it on the primary screen.
func placeCentered(hwnd uintptr, winW, winH int32) {
	scrW, _, _ := procGetSystemMetrics.Call(smCXScreen)
	scrH, _, _ := procGetSystemMetrics.Call(smCYScreen)
	x := max((int32(scrW)-winW)/2, 0)
	y := max((int32(scrH)-winH)/2, 0)
	procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(winW), uintptr(winH),
		swpFrameChange|swpNoZOrder|swpShowWindow|swpNoActivate)
}
