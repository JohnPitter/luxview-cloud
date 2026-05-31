//go:build windows

package main

// In windowed mode the Serious Engine creates its game window pinned to the
// top-left corner. We deliberately do NOT re-style it: adding a caption/border
// shifts the client area and desyncs the engine's fixed-size backbuffer, so the
// game renders OUTSIDE a black frame. Instead we only *move* the window to the
// screen center (no resize, no style change), which keeps the D3D device intact.
// The engine keeps re-pinning the window to the corner during init, so we
// re-center whenever it drifts back there.

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
	procGetClientRect            = user32.NewProc("GetClientRect")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
)

const (
	swpNoSize     = 0x0001
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
	smCXScreen    = 0
	smCYScreen    = 1
)

type winRect struct{ Left, Top, Right, Bottom int32 }

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// --- find the game window by client size (callbacks are package-level so we
// don't leak a new syscall callback on every poll iteration) ---

var (
	gwWantW, gwWantH int32
	gwSelfPID        uint32
	gwFound          uintptr
)

var gwEnumCb = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
	var wpid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&wpid)))
	if wpid == gwSelfPID {
		return 1 // skip the launcher's own window
	}
	if r, _, _ := procIsWindowVisible.Call(hwnd); r == 0 {
		return 1
	}
	var rc winRect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	w, h := rc.Right-rc.Left, rc.Bottom-rc.Top
	if abs32(w-gwWantW) <= 48 && abs32(h-gwWantH) <= 48 {
		gwFound = hwnd
		return 0
	}
	return 1
})

func findGameWindow(wantW, wantH int32) uintptr {
	gwWantW, gwWantH = wantW, wantH
	gwSelfPID = windows.GetCurrentProcessId()
	gwFound = 0
	procEnumWindows.Call(gwEnumCb, 0)
	return gwFound
}

// frameGameWindow waits for the game's main window (matching wantW×wantH) and
// centers it on screen. No-op for fullscreen. The Serious Engine keeps pinning the
// window to the top-left corner during init, so we re-center whenever it drifts
// back there for a few seconds.
func frameGameWindow(wantW, wantH int32) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var hwnd uintptr
	for range 120 { // ~60s — the driver does MD5 checks before launching
		if hwnd = findGameWindow(wantW, wantH); hwnd != 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if hwnd == 0 {
		return
	}
	centerWindow(hwnd)
	for range 24 { // ~12s of adaptive re-centering
		time.Sleep(500 * time.Millisecond)
		var rc winRect
		procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		if rc.Left < 40 && rc.Top < 40 { // engine pinned it to the corner
			centerWindow(hwnd)
		}
	}
}

// centerWindow moves the window to the center of the primary screen WITHOUT
// resizing or re-styling it. Move-only is what keeps the engine's render surface
// in sync — changing size/style desyncs the fixed-size backbuffer (black frame).
func centerWindow(hwnd uintptr) {
	var rc winRect
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	winW := rc.Right - rc.Left
	winH := rc.Bottom - rc.Top

	scrW, _, _ := procGetSystemMetrics.Call(smCXScreen)
	scrH, _, _ := procGetSystemMetrics.Call(smCYScreen)
	x := max((int32(scrW)-winW)/2, 0)
	y := max((int32(scrH)-winH)/2, 0)

	procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), 0, 0,
		swpNoSize|swpNoZOrder|swpNoActivate)
}
