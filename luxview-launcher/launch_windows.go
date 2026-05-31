//go:build windows

package main

import (
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// runGame launches exePath with the given args (space-joined) elevated via
// ShellExecute "runas" — honors a requireAdministrator manifest and triggers the
// UAC consent prompt. Used as a fallback and for the one-shot HKLM registry write.
func runGame(exePath, args, cwd string) error {
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exePath)
	dir, _ := windows.UTF16PtrFromString(cwd)
	var argp *uint16
	if args != "" {
		argp, _ = windows.UTF16PtrFromString(args)
	}
	return windows.ShellExecute(0, verb, file, argp, dir, windows.SW_SHOWNORMAL)
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
	return got == clientDir || got == clientDir+`\`
}
