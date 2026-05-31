//go:build windows

package main

import (
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Win32 helpers for two things the original launcher did:
//   1. Auto-pick the load.bin "Window Mode / FullScreen" dialog (so it doesn't
//      block — we click the button matching the user's chosen mode).
//   2. Give the windowed game a centered, draggable titled window (the Serious
//      Engine pins it to the corner with no frame).

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procEnumChildWindows         = user32.NewProc("EnumChildWindows")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procGetClientRect            = user32.NewProc("GetClientRect")
	procGetWindowLongPtr         = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtr         = user32.NewProc("SetWindowLongPtrW")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procAdjustWindowRect         = user32.NewProc("AdjustWindowRect")
	procGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
	procSetWindowText            = user32.NewProc("SetWindowTextW")
	procGetWindowText            = user32.NewProc("GetWindowTextW")
	procSendMessage              = user32.NewProc("SendMessageW")
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
	bmClick        = 0x00F5
)

type winRect struct{ Left, Top, Right, Bottom int32 }

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

func windowText(hwnd uintptr) string {
	buf := make([]uint16, 256)
	procGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf)
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

// --- find a button by its (space-stripped, lowercased) caption ---

var (
	btnKeyword string
	btnFound   uintptr
)

var btnChildCb = syscall.NewCallback(func(ch, _ uintptr) uintptr {
	t := strings.ToLower(strings.ReplaceAll(windowText(ch), " ", ""))
	if t != "" && strings.Contains(t, btnKeyword) {
		btnFound = ch
		return 0
	}
	return 1
})

var btnTopCb = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
	if r, _, _ := procIsWindowVisible.Call(hwnd); r == 0 {
		return 1
	}
	procEnumChildWindows.Call(hwnd, btnChildCb, 0)
	if btnFound != 0 {
		return 0
	}
	return 1
})

func findButton(keyword string) uintptr {
	btnKeyword = keyword
	btnFound = 0
	procEnumWindows.Call(btnTopCb, 0)
	return btnFound
}

// autoSelectDisplayMode clicks the load.bin "Window Mode / FullScreen" dialog
// button matching the user's chosen mode, so the dialog never blocks.
func autoSelectDisplayMode(fullscreen bool) {
	keyword := "windowmode"
	if fullscreen {
		keyword = "fullscreen"
	}
	for range 60 { // ~30s
		if btn := findButton(keyword); btn != 0 {
			procSendMessage.Call(btn, bmClick, 0, 0)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// frameGameWindow waits for the game's main window (matching wantW×wantH) and
// turns it into a centered, draggable titled window. No-op for fullscreen.
func frameGameWindow(wantW, wantH int32) {
	var hwnd uintptr
	for range 120 { // ~60s
		if hwnd = findGameWindow(wantW, wantH); hwnd != 0 {
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
