package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	pristonQuotedEntry = regexp.MustCompile(`(?i)("Server(?:1|2|3|Name)")\s+"[^"]*"`)
	pristonIniLine     = regexp.MustCompile(`(?im)^(ServerAddress|ServerPort|ServerName)=.*$`)
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
		return patchPristonINI(iniPath, ip, name)
	}
	return os.WriteFile(iniPath, []byte(
		"ServerAddress="+ip+"\nServerPort=10012\nServerName="+name+"\nGameExecutable=game.exe\n",
	), 0o644)
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
