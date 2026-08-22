package main

import (
	"os"
	"path/filepath"
	"regexp"
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

var muWorldDirName = regexp.MustCompile(`(?i)^World(\d+)$`)

func patchMuClient(clientDir string, card GameCard) error {
	if err := ensureMuEncTerrain(clientDir); err != nil {
		return err
	}
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

// ensureMuEncTerrain copies a donor EncTerrain.obj into Season 9 worlds that
// shipped with .map/.att but no .obj. The IGCN splash loads every listed world
// and exits with "EncTerrainN.obj file not found" otherwise.
func ensureMuEncTerrain(clientDir string) error {
	dataDir := filepath.Join(clientDir, "Data")
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	type worldInfo struct {
		dir    string
		number string
		hasObj bool
		hasMap bool
	}
	worlds := map[string]*worldInfo{}
	var donor []byte
	donorSize := int(^uint(0) >> 1)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		match := muWorldDirName.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		info := &worldInfo{dir: filepath.Join(dataDir, entry.Name()), number: match[1]}
		worlds[match[1]] = info
		files, err := os.ReadDir(info.dir)
		if err != nil {
			return err
		}
		for _, file := range files {
			name := strings.ToLower(file.Name())
			switch {
			case strings.HasPrefix(name, "encterrain") && strings.HasSuffix(name, ".obj"):
				info.hasObj = true
				path := filepath.Join(info.dir, file.Name())
				stat, err := os.Stat(path)
				if err != nil || stat.Size() == 0 {
					continue
				}
				if int(stat.Size()) >= donorSize {
					continue
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					continue
				}
				donor = raw
				donorSize = len(raw)
			case strings.HasPrefix(name, "encterrain") && strings.HasSuffix(name, ".map"):
				info.hasMap = true
			}
		}
	}
	if len(donor) == 0 {
		return nil
	}
	for _, info := range worlds {
		if !info.hasMap || info.hasObj {
			continue
		}
		dest := filepath.Join(info.dir, "EncTerrain"+info.number+".obj")
		if err := os.WriteFile(dest, donor, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// muEncodeHost is the WebZen ParameterA encoding used by official / OpenMU launchers.
func muEncodeHost(ip string) string {
	var b strings.Builder
	counter := 0
	for _, ch := range ip {
		counter++
		switch counter {
		case 1:
			b.WriteRune(ch + 12)
		case 2:
			b.WriteRune(ch + 7)
		case 3:
			b.WriteRune(ch + 3)
		case 4:
			b.WriteRune(ch + 0x13)
			counter = 0
		}
	}
	return b.String()
}

func muEncodePort(ip string, port int) int {
	switch len(ip) % 4 {
	case 0:
		return port + 12 - (((port / 4) % 4) * 8)
	case 1:
		return port + 7 - ((port % 8) * 2)
	case 2:
		return port + 3 - ((port % 4) * 2)
	default:
		return port + (0x13 - ((port % 4) * 2)) - (((port / 0x10) % 2) * 0x20)
	}
}
