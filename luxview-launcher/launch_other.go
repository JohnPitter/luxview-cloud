//go:build !windows

package main

import "os/exec"

func startGameCmd(exePath, _, cwd string) (*exec.Cmd, error) {
	cmd := exec.Command(exePath)
	cmd.Dir = cwd
	return cmd, cmd.Start()
}

func hklmLocationOK(_, _ string) bool   { return true }
func setHKCURootDir(_, _ string)        {}
func setHKLMElevated(_, _ string) error { return nil }
