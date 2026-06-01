//go:build !windows

package main

func frameGameWindow(_, _ string)      {}
func runMinimizeHotkey(_ string)       {}
func patchKeyHook(_ string)            {}
func fillScreenResolution() (int, int) { return 0, 0 }
