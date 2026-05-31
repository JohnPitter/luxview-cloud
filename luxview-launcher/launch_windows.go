//go:build windows

package main

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// gameProcessRunning reports whether a process with the given image name (e.g.
// "rakion.bin") is currently running.
func gameProcessRunning(name string) bool { return gameProcessPID(name) != 0 }

// gameProcessPID returns the PID of the first process with the given image name,
// or 0 if not running.
func gameProcessPID(name string) uint32 {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(snap)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if windows.Process32First(snap, &pe) != nil {
		return 0
	}
	for {
		if strings.EqualFold(windows.UTF16ToString(pe.ExeFile[:]), name) {
			return pe.ProcessID
		}
		if windows.Process32Next(snap, &pe) != nil {
			return 0
		}
	}
}

// startGameCmd launches the game with an EXACT command line (no exe path as the
// first token). The game reads its login from the start of the command line, so
// SysProcAttr.CmdLine guarantees it sees "user hexpass ticket" verbatim.
func startGameCmd(exePath, cmdLine, cwd string) (*exec.Cmd, error) {
	// Construct Cmd directly (not exec.Command) so we bypass LookPath — the game
	// executable is "load.bin" (a PE without a .exe extension).
	cmd := &exec.Cmd{
		Path:        exePath,
		Dir:         cwd,
		SysProcAttr: &syscall.SysProcAttr{CmdLine: cmdLine},
	}
	return cmd, cmd.Start()
}

func shellExec(verb, exe, args, cwd string, show int32) error {
	v, _ := windows.UTF16PtrFromString(verb)
	f, _ := windows.UTF16PtrFromString(exe)
	d, _ := windows.UTF16PtrFromString(cwd)
	var a *uint16
	if args != "" {
		a, _ = windows.UTF16PtrFromString(args)
	}
	return windows.ShellExecute(0, v, f, a, d, show)
}

// setHKCURootDir sets HKCU\<key>\RootDir (no admin, no window).
func setHKCURootDir(key, value string) {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, key, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer k.Close()
	_ = k.SetStringValue("RootDir", value)
}

// hklmLocationOK reports whether HKLM\<key>\Location already points at clientDir.
func hklmLocationOK(key, clientDir string) bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, key, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	got, _, err := k.GetStringValue("Location")
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimRight(got, `\`), strings.TrimRight(clientDir, `\`))
}

// setHKLMElevated writes Location+Version to HKLM via an elevated, HIDDEN
// `reg import` (no cmd window, no overwrite prompt). One UAC consent; a no-op on
// the next launch since hklmLocationOK() will then match.
func setHKLMElevated(key, clientDir string) error {
	content := "Windows Registry Editor Version 5.00\r\n\r\n" +
		"[HKEY_LOCAL_MACHINE\\" + key + "]\r\n" +
		`"Location"="` + escapeReg(clientDir+`\`) + "\"\r\n" +
		"\"Version\"=dword:00000001\r\n"
	f, err := os.CreateTemp("", "luxview-*.reg")
	if err != nil {
		return err
	}
	path := f.Name()
	_, _ = f.WriteString(content)
	_ = f.Close()
	// Left for OS temp cleanup: the elevated reg.exe reads it asynchronously.
	return shellExec("runas", "reg.exe", `import "`+path+`"`, "", windows.SW_HIDE)
}

func escapeReg(s string) string { return strings.ReplaceAll(s, `\`, `\\`) }
