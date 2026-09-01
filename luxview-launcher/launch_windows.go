//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// gameProcessRunning reports whether a process with the given image name (e.g.
// "rakion.bin") is currently running.
func gameProcessRunning(name string) bool { return gameProcessPID(name) != 0 }

// launchExecutable mirrors the known-good legacy startup command from
// JOGAR.ps1: the client must run elevated and resolve all DLL/config paths from
// its own directory.
func launchExecutable(exePath, workingDir string) error {
	if err := shellExec("runas", exePath, "", workingDir, windows.SW_SHOWNORMAL); err != nil {
		return fmt.Errorf("falha ao iniciar o jogo: %w", err)
	}
	return nil
}

// launchTibiaExecutable starts the GUI client without inheriting a console.
// OTClient is shipped as a console-subsystem binary and otherwise opens a
// visible cmd window next to the game.
//
// Credenciais vão por env (não na linha de comando): o client deriva
// user@luxviewot.com e faz auto-login + entrada no personagem.
func launchTibiaExecutable(exePath, workingDir, username, password, character string) error {
	command := exec.Command(exePath, "--luxview-autologin")
	command.Dir = workingDir
	command.Env = append(os.Environ(),
		"LUXVIEW_USER="+strings.TrimSpace(username),
		"LUXVIEW_PASSWORD="+password,
		"LUXVIEW_CHARACTER="+strings.TrimSpace(character),
	)
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar o Tibia: %w", err)
	}
	return nil
}

func launchMuClient(exePath, workingDir, ip string, port, serverID int, account, password string) error {
	if strings.TrimSpace(ip) == "" {
		return fmt.Errorf("servidor MU sem IP")
	}
	writeMuConnectionRegistry(ip, port)
	command := exec.Command(exePath, "connect", "/u"+ip, "/p"+strconv.Itoa(port))
	command.Dir = workingDir
	// Credenciais e canal vão por env (não na linha de comando): o client lê em
	// LauncherBoot::Initialize e limpa as variáveis do próprio ambiente.
	if account != "" && password != "" {
		command.Env = append(os.Environ(),
			"LUXVIEW_MU_ACCOUNT="+account,
			"LUXVIEW_MU_PASSWORD="+password,
		)
		if serverID >= 0 {
			command.Env = append(command.Env, "LUXVIEW_MU_SERVER="+strconv.Itoa(serverID))
		}
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar o MU: %w", err)
	}
	go command.Wait()
	time.Sleep(2 * time.Second)
	if !gameProcessRunning(filepath.Base(exePath)) {
		return fmt.Errorf("o MU abriu e fechou na hora — client incompleto. Clique em INSTALAR de novo")
	}
	return nil
}

func writeMuConnectionRegistry(ip string, port int) {
	host := muEncodeHost(ip)
	encodedPort := uint32(muEncodePort(ip, port))
	write := func(root registry.Key, path string) {
		key, _, err := registry.CreateKey(root, path, registry.SET_VALUE)
		if err != nil {
			return
		}
		defer key.Close()
		_ = key.SetDWordValue("Key", uint32(time.Now().Unix()))
		_ = key.SetStringValue("ParameterA", host)
		_ = key.SetDWordValue("ParameterB", encodedPort)
	}
	write(registry.CURRENT_USER, `SOFTWARE\WebZen\Mu\Connection`)
	write(registry.LOCAL_MACHINE, `SOFTWARE\WebZen\Mu\Connection`)
	write(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\WebZen\Mu\Connection`)
}

// gameProcessPID returns the PID of the first process with the given image name,
// or 0 if not running.
func gameProcessPID(name string) uint32 {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(snap)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if windows.Process32First(snap, &pe) != nil {
		return 0
	}
	for {
		if strings.EqualFold(windows.UTF16ToString(pe.ExeFile[:]), name) {
			return pe.ProcessID
		}
		if windows.Process32Next(snap, &pe) != nil {
			return 0
		}
	}
}

func shellExec(verb, exe, args, cwd string, show int32) error {
	v, _ := windows.UTF16PtrFromString(verb)
	f, _ := windows.UTF16PtrFromString(exe)
	d, _ := windows.UTF16PtrFromString(cwd)
	var a *uint16
	if args != "" {
		a, _ = windows.UTF16PtrFromString(args)
	}
	return windows.ShellExecute(0, v, f, a, d, show)
}

// setHKCURootDir sets HKCU\<key>\RootDir (no admin, no window).
func setHKCURootDir(key, value string) {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, key, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer k.Close()
	_ = k.SetStringValue("RootDir", value)
}

// hklmLocationOK reports whether HKLM\<key>\Location already points at clientDir.
func hklmLocationOK(key, clientDir string) bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, key, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	got, _, err := k.GetStringValue("Location")
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimRight(got, `\`), strings.TrimRight(clientDir, `\`))
}

// setHKLMElevated writes Location+Version to HKLM via an elevated, HIDDEN
// `reg import` (no cmd window, no overwrite prompt). One UAC consent; a no-op on
// the next launch since hklmLocationOK() will then match.
func setHKLMElevated(key, clientDir string) error {
	content := "Windows Registry Editor Version 5.00\r\n\r\n" +
		"[HKEY_LOCAL_MACHINE\\" + key + "]\r\n" +
		`"Location"="` + escapeReg(clientDir+`\`) + "\"\r\n" +
		"\"Version\"=dword:00000001\r\n"
	f, err := os.CreateTemp("", "luxview-*.reg")
	if err != nil {
		return err
	}
	path := f.Name()
	_, _ = f.WriteString(content)
	_ = f.Close()
	// Left for OS temp cleanup: the elevated reg.exe reads it asynchronously.
	return shellExec("runas", "reg.exe", `import "`+path+`"`, "", windows.SW_HIDE)
}

func escapeReg(s string) string { return strings.ReplaceAll(s, `\`, `\\`) }

const pristonRegistryKey = `SOFTWARE\WOW6432Node\Triglow Pictures\PristonTale`

// writePristonRegistry configures the v4220 client registration. The client
// reads these values from HKLM, so a failed write must be reported rather than
// launching a client that cannot connect.
func writePristonRegistry(serverIP, serverName string) error {
	serverIP = strings.TrimSpace(serverIP)
	serverName = strings.TrimSpace(serverName)
	if serverIP == "" {
		return fmt.Errorf("servidor Priston sem IP configurado")
	}
	if serverName == "" {
		serverName = "LuxView Priston"
	}
	if pristonRegistryOK(serverIP, serverName) {
		return nil
	}

	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, pristonRegistryKey, registry.SET_VALUE)
	if err == nil {
		writeErr := key.SetStringValue("ServerName", serverName)
		if writeErr == nil {
			writeErr = key.SetStringValue("Server1", serverIP)
		}
		if writeErr == nil {
			writeErr = key.SetStringValue("Server2", serverIP)
		}
		if writeErr == nil {
			writeErr = key.SetStringValue("Server3", serverIP)
		}
		if writeErr == nil {
			writeErr = key.SetStringValue("Version", "4220")
		}
		_ = key.Close()
		if writeErr == nil {
			return nil
		}
		err = writeErr
	}

	if elevatedErr := setPristonRegistryElevated(serverIP, serverName); elevatedErr != nil {
		return fmt.Errorf("permissão de administrador necessária para configurar o Priston: %w (escrita direta: %v)", elevatedErr, err)
	}
	return nil
}

func pristonRegistryOK(serverIP, serverName string) bool {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, pristonRegistryKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	gotIP, _, ipErr := key.GetStringValue("Server1")
	gotName, _, nameErr := key.GetStringValue("ServerName")
	version, _, versionErr := key.GetStringValue("Version")
	return ipErr == nil && nameErr == nil && versionErr == nil && gotIP == serverIP && gotName == serverName && version == "4220"
}

func setPristonRegistryElevated(serverIP, serverName string) error {
	content := "Windows Registry Editor Version 5.00\r\n\r\n" +
		"[HKEY_LOCAL_MACHINE\\" + pristonRegistryKey + "]\r\n" +
		`"ServerName"="` + escapeReg(serverName) + `"` + "\r\n" +
		`"Server1"="` + escapeReg(serverIP) + `"` + "\r\n" +
		`"Server2"="` + escapeReg(serverIP) + `"` + "\r\n" +
		`"Server3"="` + escapeReg(serverIP) + `"` + "\r\n" +
		`"Version"="4220"` + "\r\n"
	f, err := os.CreateTemp("", "luxview-priston-*.reg")
	if err != nil {
		return err
	}
	path := f.Name()
	if _, err = f.WriteString(content); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	if err = f.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	if err := shellExec("runas", "reg.exe", `import "`+path+`"`, "", windows.SW_HIDE); err != nil {
		return err
	}
	return nil
}
