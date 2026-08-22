package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	pristonGamePort = 10012
	pristonClanPort = 10013
)

var (
	pristonQuotedEntry = regexp.MustCompile(`(?i)("Server(?:1|2|3|Name)")\s+"[^"]*"`)
	pristonIniLine     = regexp.MustCompile(`(?im)^(ServerAddress|ServerPort|ServerName)=.*$`)
	pristonGameIniLine = regexp.MustCompile(`(?im)^(IP|Port|Clan)\s*=.*$`)
	pristonBPTConnect  = []byte("189.46.228.170:30303")
)

func pristonExecutable(clientDir string) string {
	for _, name := range []string{"game.exe", "Game.exe", "SunnyBPT.exe", "psupdate.exe"} {
		path := filepath.Join(clientDir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func patchPristonClient(clientDir string, card GameCard) error {
	ip := strings.TrimSpace(card.ServerIP)
	if ip == "" {
		return nil
	}
	name := strings.TrimSpace(card.DisplayName)
	if name == "" {
		name = "LuxView"
	}
	if err := patchPristonReg(filepath.Join(clientDir, "ptReg.rgx"), ip, name); err != nil && !os.IsNotExist(err) {
		return err
	}
	iniPath := filepath.Join(clientDir, "openpriston.launcher.ini")
	if _, err := os.Stat(iniPath); err == nil {
		if err := patchPristonINI(iniPath, ip, name); err != nil {
			return err
		}
	} else {
		if err := os.WriteFile(iniPath, []byte(
			"ServerAddress="+ip+"\nServerPort="+strconv.Itoa(pristonGamePort)+"\nServerName="+name+"\nGameExecutable=game.exe\n",
		), 0o644); err != nil {
			return err
		}
	}
	gameIni := filepath.Join(clientDir, "game.ini")
	if _, err := os.Stat(gameIni); err == nil {
		if err := patchPristonGameINI(gameIni, ip); err != nil {
			return err
		}
	}
	for _, exeName := range []string{"Game.exe", "game.exe"} {
		exe := filepath.Join(clientDir, exeName)
		if err := patchPristonGameExe(exe, ip); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func patchPristonReg(path, ip, name string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated := pristonQuotedEntry.ReplaceAllFunc(content, func(match []byte) []byte {
		key := pristonQuotedEntry.FindSubmatch(match)[1]
		value := ip
		if strings.EqualFold(string(key), `"ServerName"`) {
			value = name
		}
		return []byte(string(key) + ` "` + value + `"`)
	})
	if string(updated) == string(content) {
		return nil
	}
	return os.WriteFile(path, updated, 0o644)
}

func patchPristonINI(path, ip, name string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	values := map[string]string{
		"ServerAddress": ip,
		"ServerPort":    strconv.Itoa(pristonGamePort),
		"ServerName":    name,
	}
	updated := pristonIniLine.ReplaceAllFunc(content, func(match []byte) []byte {
		parts := strings.SplitN(string(match), "=", 2)
		if len(parts) == 0 {
			return match
		}
		if value, ok := values[parts[0]]; ok {
			return []byte(parts[0] + "=" + value)
		}
		return match
	})
	return os.WriteFile(path, updated, 0o644)
}

func patchPristonGameINI(path, ip string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	values := map[string]string{
		"IP":   ip,
		"Port": strconv.Itoa(pristonGamePort),
		"Clan": ip + ":" + strconv.Itoa(pristonClanPort),
	}
	updated := pristonGameIniLine.ReplaceAllFunc(content, func(match []byte) []byte {
		parts := strings.SplitN(string(match), "=", 2)
		if len(parts) == 0 {
			return match
		}
		key := strings.TrimSpace(parts[0])
		value, ok := values[key]
		if !ok {
			return match
		}
		return []byte(key + "=" + value)
	})
	return os.WriteFile(path, updated, 0o644)
}

func patchPristonGameExe(path, ip string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	replacement := ip + ":" + strconv.Itoa(pristonGamePort)
	if len(replacement) > len(pristonBPTConnect) {
		return nil
	}
	padded := make([]byte, len(pristonBPTConnect))
	copy(padded, replacement)
	updated := bytes.ReplaceAll(content, pristonBPTConnect, padded)
	if bytes.Equal(updated, content) {
		return nil
	}
	return os.WriteFile(path, updated, 0o644)
}
