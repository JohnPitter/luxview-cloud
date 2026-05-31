//go:build !windows

package main

import "fmt"

func runGame(_, _, _ string) error {
	return fmt.Errorf("o jogo só roda no Windows")
}

func hklmLocationOK(_, _ string) bool   { return true }
func setHKCURootDir(_, _ string)        {}
func setHKLMElevated(_, _ string) error { return nil }
