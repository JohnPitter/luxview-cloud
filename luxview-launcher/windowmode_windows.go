//go:build windows

package main

import (
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Camada de "modo janela": a Serious Engine (2007) cria a janela presa no canto
// e sem moldura. Depois que o jogo abre, achamos a janela dele e damos uma barra
// de título arrastável + centralizamos na tela.

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procGetClientRect            = user32.NewProc("GetClientRect")
	procGetWindowLongPtr         = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtr         = user32.NewProc("SetWindowLongPtrW")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procAdjustWindowRect         = user32.NewProc("AdjustWindowRect")
	procGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
	procSetWindowText            = user32.NewProc("SetWindowTextW")
)

const (
	gwlStyle       = ^uintptr(15) // -16
	wsPopup        = 0x80000000
	wsBorder       = 0x00800000
	wsDlgFrame     = 0x00400000
	wsCaption      = wsBorder | wsDlgFrame // 0x00C00000
	wsSysMenu      = 0x00080000
	wsMinimizeBox  = 0x00020000
	wsThickFrame   = 0x00040000
	swpNoZOrder    = 0x0004
	swpNoActivate  = 0x0010
	swpFrameChange = 0x0020
	swpShowWindow  = 0x0040
	smCXScreen     = 0
	smCYScreen     = 1
)

type winRect struct{ Left, Top, Right, Bottom int32 }

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

func findWindowForPID(pid uint32, wantW, wantH int32) uintptr {
	var found uintptr
	cb := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		var wpid uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&wpid)))
		if wpid != pid {
			return 1
		}
		if r, _, _ := procIsWindowVisible.Call(hwnd); r == 0 {
			return 1
		}
		var rc winRect
		procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		w, h := rc.Right-rc.Left, rc.Bottom-rc.Top
		if abs32(w-wantW) <= 48 && abs32(h-wantH) <= 48 {
			found = hwnd
			return 0 // stop enumeration
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
	return found
}

// frameGameWindow waits for the game's main window (matching wantW×wantH) and
// turns it into a centered, draggable titled window. No-op for fullscreen.
func frameGameWindow(pid uint32, wantW, wantH int32) {
	var hwnd uintptr
	for range 120 { // ~60s
		if hwnd = findWindowForPID(pid, wantW, wantH); hwnd != 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if hwnd == 0 {
		return
	}
	// Apply twice (the engine may reposition itself right after creating the window).
	applyWindowedFrame(hwnd, wantW, wantH)
	time.Sleep(1500 * time.Millisecond)
	applyWindowedFrame(hwnd, wantW, wantH)
}

func applyWindowedFrame(hwnd uintptr, clientW, clientH int32) {
	style, _, _ := procGetWindowLongPtr.Call(hwnd, gwlStyle)
	style &^= wsPopup
	style |= wsCaption | wsSysMenu | wsMinimizeBox | wsThickFrame
	procSetWindowLongPtr.Call(hwnd, gwlStyle, style)

	rc := winRect{0, 0, clientW, clientH}
	procAdjustWindowRect.Call(uintptr(unsafe.Pointer(&rc)), style, 0)
	winW := rc.Right - rc.Left
	winH := rc.Bottom - rc.Top

	scrW, _, _ := procGetSystemMetrics.Call(smCXScreen)
	scrH, _, _ := procGetSystemMetrics.Call(smCYScreen)
	x := (int32(scrW) - winW) / 2
	y := max((int32(scrH)-winH)/2, 0)

	procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(winW), uintptr(winH),
		swpNoZOrder|swpFrameChange|swpShowWindow|swpNoActivate)

	title, _ := windows.UTF16PtrFromString("Rakion — LuxView Cloud Games")
	procSetWindowText.Call(hwnd, uintptr(unsafe.Pointer(title)))
}
