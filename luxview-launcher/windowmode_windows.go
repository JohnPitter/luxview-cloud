//go:build windows

package main

// In windowed mode we center the game's render window on the primary screen.
// We identify it by the game process (rakion.bin) AND window class "Rakion" — the
// Serious Engine render window — so we never grab its "Loading..." splash or any
// unrelated app window. We only MOVE it (no resize/restyle): changing size/style
// desyncs the engine's fixed-size backbuffer (black frame). The game is a legacy
// DPI-UNAWARE process while this launcher is per-monitor DPI-AWARE, so we run the
// window math on a thread set to DPI-UNAWARE to share the game's coordinate space
// on scaled displays.
//
// Note: third-party game overlays (Discord/NVIDIA) only render correctly over this
// game in EXCLUSIVE FULLSCREEN — in windowed/borderless they fall back to a black
// top-most layer. That's an overlay limitation with this legacy DX9 title, outside
// the launcher's control; fullscreen is the default for that reason.

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
	procGetClassName             = user32.NewProc("GetClassNameW")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
	procSetThreadDpiAwarenessCtx = user32.NewProc("SetThreadDpiAwarenessContext")
)

const (
	swpNoSize     = 0x0001
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
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

// frameGameWindow waits for the game's render window and centers it on the primary
// screen. No-op for fullscreen. The engine keeps re-pinning the window to the
// top-left corner during init, so we re-center while it drifts back there.
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
	// The render window starts at 1x1; wait for its final size before centering.
	for range 40 {
		if w, h := windowSize(hwnd); w >= 320 && h >= 240 {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	centerWindow(hwnd)
	for range 24 { // re-center while the engine pins it to the corner during init
		time.Sleep(500 * time.Millisecond)
		var rc winRect
		procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		if rc.Left < 40 && rc.Top < 40 {
			centerWindow(hwnd)
		}
	}
}

func windowSize(hwnd uintptr) (int32, int32) {
	var rc winRect
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	return rc.Right - rc.Left, rc.Bottom - rc.Top
}

// centerWindow moves the window to the center of the primary screen WITHOUT
// resizing or re-styling it (move-only keeps the engine's render surface in sync).
func centerWindow(hwnd uintptr) {
	winW, winH := windowSize(hwnd)
	if winW <= 0 || winH <= 0 {
		return
	}
	scrW, _, _ := procGetSystemMetrics.Call(smCXScreen)
	scrH, _, _ := procGetSystemMetrics.Call(smCYScreen)
	x := max((int32(scrW)-winW)/2, 0)
	y := max((int32(scrH)-winH)/2, 0)
	procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), 0, 0,
		swpNoSize|swpNoZOrder|swpNoActivate)
}
