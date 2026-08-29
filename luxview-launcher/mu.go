package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	muDefaultConnectPort = 44405
	muDefaultChatPort    = 55980
)

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

// muServerNames maps the ConnectServer group index (ServerId/20) to the
// friendly name of that physical server, matching config."GameServerDefinition"
// on the OpenMU database (Description column: "LuxMu (S6)"/"LuxMu (99d)"/
// "LuxMu (S2)"). A future new server without an entry here falls back to
// "Servidor N" instead of failing.
var muServerNames = map[int]string{
	0: "Season 6 Pt 3",
	1: "Season 99d",
	2: "Season 2",
}

// MuServerInfo is one game server entry reported by the MU ConnectServer.
// ConnectServer packs two dimensions into a single ServerId: Server (id/20)
// identifies which physical game server, and Channel (id%20) identifies a
// channel within that server. Today every server exposes exactly one channel,
// but the picker groups by Server so a future multi-channel server renders as
// the server name with several "Canal M" rows instead of looking like N
// separate servers.
type MuServerInfo struct {
	ID      int    `json:"id"`
	Server  int    `json:"server"`
	Channel int    `json:"channel"`
	Name    string `json:"name"`
	Load    int    `json:"load"`
}

// queryMuServers asks the MU ConnectServer for its live server list using the
// same F4.06 handshake the game client uses, so the launcher can offer channel
// selection before starting main.exe.
func queryMuServers(host string, port int) ([]MuServerInfo, error) {
	address := net.JoinHostPort(strings.TrimSpace(host), strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", address, 8*time.Second)
	if err != nil {
		return nil, fmt.Errorf("não consegui contatar o ConnectServer: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(8 * time.Second))

	// Server hello (C1 04 00 01) — read and ignore it.
	if _, err := io.ReadFull(conn, make([]byte, 4)); err != nil {
		return nil, fmt.Errorf("ConnectServer não respondeu: %w", err)
	}
	if _, err := conn.Write([]byte{0xC1, 0x04, 0xF4, 0x06}); err != nil {
		return nil, fmt.Errorf("falha ao pedir a lista de canais: %w", err)
	}

	header := make([]byte, 7)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, fmt.Errorf("resposta inválida do ConnectServer: %w", err)
	}
	if header[0] != 0xC2 || header[3] != 0xF4 || header[4] != 0x06 {
		return nil, fmt.Errorf("resposta inesperada do ConnectServer")
	}
	length := int(binary.BigEndian.Uint16(header[1:3]))
	count := int(binary.BigEndian.Uint16(header[5:7]))
	if length < 7+count*4 || count > 256 {
		return nil, fmt.Errorf("lista de canais inválida")
	}
	body := make([]byte, length-7)
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, fmt.Errorf("lista de canais incompleta: %w", err)
	}

	servers := make([]MuServerInfo, 0, count)
	for i := 0; i < count; i++ {
		id := int(binary.LittleEndian.Uint16(body[i*4 : i*4+2]))
		load := int(body[i*4+2])
		group := id / 20
		server := group + 1
		channel := id%20 + 1
		name, known := muServerNames[group]
		if !known {
			name = fmt.Sprintf("Servidor %d", server)
		}
		servers = append(servers, MuServerInfo{
			ID:      id,
			Server:  server,
			Channel: channel,
			Name:    name,
			Load:    load,
		})
	}
	sort.Slice(servers, func(i, j int) bool { return servers[i].ID < servers[j].ID })
	return servers, nil
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
	if err := os.WriteFile(muLauncherConfigPath(clientDir), []byte(xml.String()), 0o644); err != nil {
		return err
	}
	if err := patchMuServerInfoFiles(clientDir, ip, muDefaultConnectPort, muDefaultChatPort); err != nil {
		return err
	}
	return patchMuPackedIPFiles(clientDir, ip)
}

var muPackedDefaultIP = []byte("192.168.0.168")

func patchMuPackedIP(raw []byte, ip string) []byte {
	repl := []byte(ip)
	if len(repl) == 0 || len(repl) > len(muPackedDefaultIP) || !bytes.Contains(raw, muPackedDefaultIP) {
		return raw
	}
	padded := make([]byte, len(muPackedDefaultIP))
	copy(padded, repl)
	return bytes.ReplaceAll(raw, muPackedDefaultIP, padded)
}

func patchMuPackedIPFiles(clientDir, ip string) error {
	for _, name := range []string{"main.exe", "Main.exe", "IGC.dll", "igc.dll"} {
		path := filepath.Join(clientDir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		patched := patchMuPackedIP(raw, ip)
		if bytes.Equal(raw, patched) {
			continue
		}
		if err := os.WriteFile(path, patched, 0o644); err != nil {
			return err
		}
	}
	return nil
}

var (
	muServerInfoIP       = regexp.MustCompile(`(?i)IP="[^"]*"`)
	muServerInfoPort     = regexp.MustCompile(`(?im)^Port=\d+`)
	muServerInfoChatPort = regexp.MustCompile(`(?im)^ChatPort=\d+`)
)

func patchMuServerInfo(raw []byte, ip string, port, chatPort int) []byte {
	out := muServerInfoIP.ReplaceAll(raw, []byte(`IP="`+ip+`"`))
	out = muServerInfoChatPort.ReplaceAll(out, []byte("ChatPort="+strconv.Itoa(chatPort)))
	out = muServerInfoPort.ReplaceAll(out, []byte("Port="+strconv.Itoa(port)))
	return out
}

func patchMuServerInfoFiles(clientDir, ip string, port, chatPort int) error {
	var files []string
	root := filepath.Join(clientDir, "Data")
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.EqualFold(d.Name(), "ServerInfo.bmd") {
			files = append(files, path)
		}
		return nil
	})
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		patched := patchMuServerInfo(raw, ip, port, chatPort)
		if bytes.Equal(raw, patched) {
			continue
		}
		if err := os.WriteFile(path, patched, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

var muMapXorTab = [...]byte{
	0xd1, 0x73, 0x52, 0xf6, 0xd2, 0x9a, 0xcb, 0x27,
	0x3e, 0xaf, 0x59, 0x31, 0x37, 0xb3, 0xe7, 0xa2,
}

func muDecryptMapFile(in []byte) []byte {
	out := make([]byte, len(in))
	key := byte(0x5e)
	for i, encode := range in {
		out[i] = (encode ^ muMapXorTab[i%len(muMapXorTab)]) - key
		key = encode + 0x3d
	}
	return out
}

func muEncryptMapFile(in []byte) []byte {
	out := make([]byte, len(in))
	key := byte(0x5e)
	for i, plain := range in {
		out[i] = (plain + key) ^ muMapXorTab[i%len(muMapXorTab)]
		key = out[i] + 0x3d
	}
	return out
}

func muRetagEncTerrain(raw []byte, world int) []byte {
	if world < 0 || world > 0xffff || len(raw) < 4 {
		return raw
	}
	plain := muDecryptMapFile(raw)
	if len(plain) < 4 {
		return raw
	}
	got := int(plain[0])<<8 | int(plain[1])
	if got == world {
		return raw
	}
	plain[0] = byte(world >> 8)
	plain[1] = byte(world)
	return muEncryptMapFile(plain)
}

// ensureMuEncTerrain copies a donor EncTerrain.obj into Season 9 worlds that
// shipped with .map/.att but no .obj, and rewrites the world id inside the
// encrypted header. Copying World79 onto World95 without that retag makes
// IGCN report "EncTerrain95.obj file corrupted".
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
	for _, info := range worlds {
		world, err := strconv.Atoi(info.number)
		if err != nil {
			continue
		}
		dest := filepath.Join(info.dir, "EncTerrain"+info.number+".obj")
		if info.hasObj {
			raw, err := os.ReadFile(dest)
			if err != nil {
				return err
			}
			fixed := muRetagEncTerrain(raw, world)
			if !bytes.Equal(fixed, raw) {
				if err := os.WriteFile(dest, fixed, 0o644); err != nil {
					return err
				}
			}
			continue
		}
		if !info.hasMap || len(donor) == 0 {
			continue
		}
		if err := os.WriteFile(dest, muRetagEncTerrain(donor, world), 0o644); err != nil {
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
