//go:build windows

package main

import (
	"runtime"
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
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procGetWindowLongPtr         = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtr         = user32.NewProc("SetWindowLongPtrW")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procAdjustWindowRect         = user32.NewProc("AdjustWindowRect")
	procGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
	procSetWindowText            = user32.NewProc("SetWindowTextW")
	procGetWindowText            = user32.NewProc("GetWindowTextW")
	procSendMessage              = user32.NewProc("SendMessageW")
	procSetWinEventHook          = user32.NewProc("SetWinEventHook")
	procUnhookWinEvent           = user32.NewProc("UnhookWinEvent")
	procGetMessage               = user32.NewProc("GetMessageW")
	procTranslateMessage         = user32.NewProc("TranslateMessage")
	procDispatchMessage          = user32.NewProc("DispatchMessageW")
	procPostThreadMessage        = user32.NewProc("PostThreadMessageW")

	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procGetCurrentThreadId = kernel32.NewProc("GetCurrentThreadId")
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
	swpNoSize      = 0x0001

	eventObjectCreate       = 0x8000
	eventObjectShow         = 0x8002
	eventObjectLocationChng = 0x800B
	objidWindow             = 0
	wineventOutOfContext    = 0x0000
	wmQuit                  = 0x0012
	offScreenXY             = uintptr(0xFFFF8300) // -32000 (low 32 bits)
)

type win32msg struct {
	hwnd           uintptr
	message        uint32
	_              uint32
	wParam, lParam uintptr
	time           uint32
	ptX, ptY       int32
}

// suppressLoadBinDialog moves load.bin's "Window Mode / FullScreen" dialog
// OFF-SCREEN the instant it's created/shown — before it paints — so it never
// appears. The button is still clicked (off-screen) by autoSelectDisplayMode, so
// load.bin processes the choice and launches the game. Scoped to load.bin's
// process; runs ~25s then unhooks. (Hiding the window breaks the click; moving
// it off-screen keeps it "visible" so the click still works.)
func suppressLoadBinDialog(pid uint32) {
	if pid == 0 {
		return
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	cb := syscall.NewCallback(func(_, event, hwnd, idObject, _, _, _ uintptr) uintptr {
		switch event {
		case eventObjectCreate, eventObjectShow, eventObjectLocationChng:
		default:
			return 0
		}
		if idObject != objidWindow || hwnd == 0 {
			return 0
		}
		var rc winRect
		procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		w, h := rc.Right-rc.Left, rc.Bottom-rc.Top
		// dialog-sized AND currently on-screen -> shove it off-screen (the > -10000
		// check avoids re-moving (looping) a window we already moved away).
		if w > 120 && w < 520 && h > 70 && h < 360 && rc.Left > -10000 {
			procSetWindowPos.Call(hwnd, 0, offScreenXY, offScreenXY, 0, 0, swpNoSize|swpNoZOrder|swpNoActivate)
		}
		return 0
	})
	hook, _, _ := procSetWinEventHook.Call(
		eventObjectCreate, eventObjectLocationChng, 0, cb, uintptr(pid), 0, wineventOutOfContext)
	if hook == 0 {
		return
	}
	defer procUnhookWinEvent.Call(hook)

	tid, _, _ := procGetCurrentThreadId.Call()
	go func() {
		time.Sleep(25 * time.Second)
		procPostThreadMessage.Call(tid, wmQuit, 0, 0)
	}()

	var msg win32msg
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

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
	for range 3000 { // ~30s, polled very fast so the dialog shows for ~1 frame
		if btn := findButton(keyword); btn != 0 {
			// NOTE: don't hide/move the dialog before clicking — load.bin then
			// fails to process the click and the game never launches. Just click.
			procSendMessage.Call(btn, bmClick, 0, 0)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// frameGameWindow waits for the game's main window (matching wantW×wantH) and
// turns it into a centered, draggable titled window. No-op for fullscreen.
// The Serious Engine keeps pinning the window to the top-left corner during
// init, so we re-center whenever it drifts there for a few seconds.
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
	applyWindowedFrame(hwnd, wantW, wantH)
	for range 24 { // ~12s of adaptive re-centering
		time.Sleep(500 * time.Millisecond)
		var rc winRect
		procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		if rc.Left < 40 && rc.Top < 40 { // engine pinned it to the corner
			centerWindow(hwnd, wantW, wantH)
		}
	}
}

// applyWindowedFrame gives the window a caption + resizable frame, sets its
// title, and centers it.
func applyWindowedFrame(hwnd uintptr, clientW, clientH int32) {
	style, _, _ := procGetWindowLongPtr.Call(hwnd, gwlStyle)
	style &^= wsPopup
	style |= wsCaption | wsSysMenu | wsMinimizeBox | wsThickFrame
	procSetWindowLongPtr.Call(hwnd, gwlStyle, style)

	title, _ := windows.UTF16PtrFromString("Rakion — LuxView Cloud Games")
	procSetWindowText.Call(hwnd, uintptr(unsafe.Pointer(title)))

	centerWindow(hwnd, clientW, clientH)
}

// centerWindow positions the window (current style) centered on the primary
// screen, sized to fit a clientW×clientH client area.
func centerWindow(hwnd uintptr, clientW, clientH int32) {
	style, _, _ := procGetWindowLongPtr.Call(hwnd, gwlStyle)
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
}
