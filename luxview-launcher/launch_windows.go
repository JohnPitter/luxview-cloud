//go:build windows

package main

import (
	"os"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

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

// runGame launches the game (elevated if its manifest demands it, visible).
func runGame(exePath, args, cwd string) error {
	return shellExec("runas", exePath, args, cwd, windows.SW_SHOWNORMAL)
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
