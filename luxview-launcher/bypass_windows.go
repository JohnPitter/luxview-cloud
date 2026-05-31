//go:build windows

package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

//go:embed rakion-drive.ps1
var rakionDriveScript string

// invokeRakionDriver launches Rakion WITHOUT the "Window Mode / FullScreen"
// dialog. load.bin is the SoftNyx RakionLauncher (a .NET assembly MPRESS-packed);
// it shows that dialog. Instead of running it, we run a 32-bit PowerShell driver
// that unpacks load.bin in memory, instantiates its Form1 INVISIBLY with the
// chosen mode pre-selected, and runs the ORIGINAL launch pipeline (login + config
// decrypt + CreateProcess(rakion.bin) + the staged GameGuard memory patches) —
// so the dialog never appears. We reuse the original code (reliable) rather than
// reimplementing the fragile GameGuard patch timing ourselves.
//
// 32-bit because load.bin is x86; elevated because the launcher manifest elevates
// us and rakion.bin requires admin.
func invokeRakionDriver(clientDir, user, hexPass string, windowed bool) error {
	base, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	script := filepath.Join(base, "LuxViewLauncher", "rakion-drive.ps1")
	if err := os.WriteFile(script, []byte(rakionDriveScript), 0o644); err != nil {
		return err
	}

	ps := filepath.Join(os.Getenv("WINDIR"), "SysWOW64", "WindowsPowerShell", "v1.0", "powershell.exe")
	w := "0"
	if windowed {
		w = "1"
	}
	cmd := exec.Command(ps, "-ExecutionPolicy", "Bypass", "-NoProfile", "-WindowStyle", "Hidden",
		"-File", script, "-ClientDir", clientDir, "-User", user, "-HexPass", hexPass, "-Windowed", w)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar o driver do Rakion: %w", err)
	}
	return nil
}
