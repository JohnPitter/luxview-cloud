//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// patchKeyHook removes the game's Alt+Tab / Windows-key lock at runtime.
//
// The client ships keyhook.dll (preferred base 0x34000000, no ASLR), which installs
// two WH_KEYBOARD_LL hooks. Their hook procedures SWALLOW Alt+Tab (VK_TAB with the
// ALTDOWN flag), Alt+Esc and the Windows keys by returning 1 (block). Statically
// confirmed: each block site is `mov eax, 1` (B8 01 00 00 00) — at RVA 0x106D and
// 0x10A2 — so the "1" immediate sits at RVA 0x106E / 0x10A3. We flip those to 0
// (`mov eax, 0`); the hook then returns 0 and the key passes through → Alt+Tab works.
//
// We patch the hook PROCEDURE (not the install call), so timing is irrelevant: as
// long as keyhook.dll is loaded, the patched bytes are on the live code path. With
// GameGuard dead (its server is offline + the staged patches make it non-fatal),
// touching game memory is safe — same approach as the existing GameGuard patches.
func patchKeyHook(processName string) {
	if processName == "" {
		return
	}
	// Wait for the game process, then for keyhook.dll to be mapped into it.
	var pid uint32
	for range 240 {
		if pid = gameProcessPID(processName); pid != 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if pid == 0 {
		return
	}
	var base uintptr
	for range 120 {
		if base = moduleBaseAddr(pid, "keyhook.dll"); base != 0 {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if base == 0 {
		patchLog("keyhook.dll não encontrada no processo — Alt+Tab não destravado")
		return
	}

	h, err := windows.OpenProcess(windows.PROCESS_VM_OPERATION|windows.PROCESS_VM_READ|windows.PROCESS_VM_WRITE, false, pid)
	if err != nil {
		patchLog(fmt.Sprintf("OpenProcess falhou: %v", err))
		return
	}
	defer windows.CloseHandle(h)

	// The two `mov eax,1` block sites (the immediate byte), relative to keyhook base.
	done := 0
	for _, rva := range []uintptr{0x106E, 0x10A3} {
		if patchByteToZero(h, base+rva) {
			done++
		}
	}
	patchLog(fmt.Sprintf("keyhook @ 0x%X — %d/2 sites destravados (Alt+Tab liberado)", base, done))
}

// moduleBaseAddr returns the load base of a named module in a process (handles a
// 32-bit module from a 64-bit caller via TH32CS_SNAPMODULE32).
func moduleBaseAddr(pid uint32, name string) uintptr {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPMODULE|windows.TH32CS_SNAPMODULE32, pid)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(snap)
	var me windows.ModuleEntry32
	me.Size = uint32(unsafe.Sizeof(me))
	if windows.Module32First(snap, &me) != nil {
		return 0
	}
	for {
		if strings.EqualFold(windows.UTF16ToString(me.Module[:]), name) {
			return me.ModBaseAddr
		}
		if windows.Module32Next(snap, &me) != nil {
			return 0
		}
	}
}

// patchByteToZero writes 0x00 at addr only if it currently reads 0x01 — idempotent,
// and it won't corrupt an unexpected build. Returns true if the site ends up 0x00.
func patchByteToZero(h windows.Handle, addr uintptr) bool {
	var cur byte
	var n uintptr
	if err := windows.ReadProcessMemory(h, addr, &cur, 1, &n); err != nil || n != 1 {
		return false
	}
	if cur == 0x00 {
		return true // already patched
	}
	if cur != 0x01 {
		return false // unexpected byte — leave it alone
	}
	var old uint32
	if err := windows.VirtualProtectEx(h, addr, 1, windows.PAGE_EXECUTE_READWRITE, &old); err != nil {
		return false
	}
	var zero byte
	var w uintptr
	werr := windows.WriteProcessMemory(h, addr, &zero, 1, &w)
	var tmp uint32
	windows.VirtualProtectEx(h, addr, 1, old, &tmp)
	return werr == nil && w == 1
}

// patchLog appends a line to %APPDATA%/LuxViewLauncher/launcher.log (best effort), so
// a failed unlock can be diagnosed remotely.
func patchLog(msg string) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(cfg, "LuxViewLauncher", "launcher.log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[keyhook] %s\n", msg)
}
