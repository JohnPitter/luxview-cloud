package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const muDefaultConnectPort = 44405

func muExecutable(clientDir string) string {
	for _, name := range []string{"main.exe", "Main.exe", "Mu.exe"} {
		path := filepath.Join(clientDir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func muConnectPort(_ GameCard) int {
	return muDefaultConnectPort
}

func muLauncherConfigPath(clientDir string) string {
	return filepath.Join(clientDir, "launcher.config")
}

func patchMuClient(clientDir string, card GameCard) error {
	ip := strings.TrimSpace(card.ServerIP)
	if ip == "" {
		return nil
	}
	name := strings.TrimSpace(card.DisplayName)
	if name == "" {
		name = "MU Online"
	}
	xml := strings.Builder{}
	xml.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	xml.WriteString("<LauncherSettings>\n")
	xml.WriteString("  <MainExePath>main.exe</MainExePath>\n")
	xml.WriteString("  <Hosts>\n")
	xml.WriteString("    <ServerHostSettings>\n")
	xml.WriteString("      <Description>" + xmlEscape(name) + "</Description>\n")
	xml.WriteString("      <Address>" + xmlEscape(ip) + "</Address>\n")
	xml.WriteString("      <Port>" + strconv.Itoa(muDefaultConnectPort) + "</Port>\n")
	xml.WriteString("    </ServerHostSettings>\n")
	xml.WriteString("  </Hosts>\n")
	xml.WriteString("</LauncherSettings>\n")
	return os.WriteFile(muLauncherConfigPath(clientDir), []byte(xml.String()), 0o644)
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
