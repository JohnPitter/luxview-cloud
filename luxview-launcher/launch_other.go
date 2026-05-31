//go:build !windows

package main

import "fmt"

func runGame(_, _ string) error {
	return fmt.Errorf("o launcher dos jogos só roda no Windows")
}
