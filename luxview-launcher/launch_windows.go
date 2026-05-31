//go:build windows

package main

import "golang.org/x/sys/windows"

// runGame launches the game's launcher elevated. Legacy clients (Rakion's
// NyxLauncher) ship a manifest that requires administrator rights, so a plain
// CreateProcess fails with "requires elevation". ShellExecute with the "runas"
// verb honors the manifest and triggers the UAC consent prompt.
func runGame(exePath, cwd string) error {
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exePath)
	dir, _ := windows.UTF16PtrFromString(cwd)
	return windows.ShellExecute(0, verb, file, nil, dir, windows.SW_SHOWNORMAL)
}
