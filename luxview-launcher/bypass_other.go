//go:build !windows

package main

import "fmt"

func invokeRakionDriver(_, _, _ string, _ bool) error {
	return fmt.Errorf("Rakion só roda no Windows")
}
