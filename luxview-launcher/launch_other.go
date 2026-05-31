//go:build !windows

package main

func hklmLocationOK(_, _ string) bool   { return true }
func setHKCURootDir(_, _ string)        {}
func setHKLMElevated(_, _ string) error { return nil }
func gameProcessRunning(_ string) bool  { return false }
func gameProcessPID(_ string) uint32    { return 0 }
