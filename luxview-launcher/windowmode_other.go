//go:build !windows

package main

func frameGameWindow(_, _ int32)       {}
func autoSelectDisplayMode(_ bool)     {}
func suppressDisplayModeDialog(_ bool) {}
