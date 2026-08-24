//go:build !windows

package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func hklmLocationOK(_, _ string) bool        { return true }
func setHKCURootDir(_, _ string)             {}
func setHKLMElevated(_, _ string) error      { return nil }
func writePristonRegistry(_, _ string) error { return nil }
func gameProcessRunning(_ string) bool       { return false }
func gameProcessPID(_ string) uint32         { return 0 }

func launchExecutable(exePath, workingDir string) error {
	command := exec.Command(exePath)
	command.Dir = workingDir
	if err := command.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar o jogo: %w", err)
	}
	return nil
}

func launchTibiaExecutable(exePath, workingDir string) error {
	return launchExecutable(exePath, workingDir)
}

func launchMuClient(exePath, workingDir, ip string, port int) error {
	if strings.TrimSpace(ip) == "" {
		return fmt.Errorf("servidor MU sem IP")
	}
	command := exec.Command(exePath, "connect", "/u"+ip, "/p"+strconv.Itoa(port))
	command.Dir = workingDir
	if err := command.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar o MU: %w", err)
	}
	return nil
}
