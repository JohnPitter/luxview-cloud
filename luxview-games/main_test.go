package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)

func TestSaveConfigQuotesShellValuesAndLoadConfigRestoresThem(t *testing.T) {
	dataDir := t.TempDir()
	gs := &GameServer{
		DataDir: dataDir,
		ConfigFields: []ConfigField{
			{Key: "VRISING_SERVER_NAME", Type: "text"},
			{Key: "VRISING_DESCRIPTION", Type: "text"},
			{Key: "VRISING_MAX_USERS", Type: "number"},
			{Key: "VRISING_PRESET", Type: "select"},
		},
	}
	values := map[string]string{
		"VRISING_SERVER_NAME": "Pseudo Gamers",
		"VRISING_DESCRIPTION": "Servidor do João",
		"VRISING_MAX_USERS":   "40",
		"VRISING_PRESET":      "StandardPvP",
	}

	if err := saveConfig(gs, values); err != nil {
		t.Fatalf("saveConfig() error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dataDir, "server-config.env"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(raw)
	if strings.Contains(content, "VRISING_SERVER_NAME=Pseudo Gamers") {
		t.Fatalf("server-config.env contains unquoted value: %q", content)
	}
	if !strings.Contains(content, "VRISING_SERVER_NAME='Pseudo Gamers'") {
		t.Fatalf("server-config.env missing quoted server name: %q", content)
	}

	cfg := loadConfig(gs)
	for key, want := range values {
		if got := cfg[key]; got != want {
			t.Fatalf("loadConfig()[%s] = %q, want %q", key, got, want)
		}
	}
}

func TestShellQuoteEscapesApostrophes(t *testing.T) {
	const value = "João's Server"

	quoted := shellQuote(value)
	if quoted != `'João'\''s Server'` {
		t.Fatalf("shellQuote() = %q", quoted)
	}
	if got := parseShellValue(quoted); got != value {
		t.Fatalf("parseShellValue() = %q, want %q", got, value)
	}
}

func TestCalculateCPUPercent(t *testing.T) {
	stats := container.StatsResponse{
		Stats: container.Stats{
			CPUStats: container.CPUStats{
				CPUUsage:    container.CPUUsage{TotalUsage: 300},
				SystemUsage: 1000,
				OnlineCPUs:  4,
			},
			PreCPUStats: container.CPUStats{
				CPUUsage:    container.CPUUsage{TotalUsage: 100},
				SystemUsage: 500,
			},
		},
	}

	if got := calculateCPUPercent(stats); got != 160 {
		t.Fatalf("calculateCPUPercent() = %v, want 160", got)
	}
}

func TestCPULimitCoresUsesNanoCPUs(t *testing.T) {
	resources := container.Resources{NanoCPUs: 750_000_000}

	if got := cpuLimitCores(resources); got != 0.75 {
		t.Fatalf("cpuLimitCores() = %v, want 0.75", got)
	}
}

func TestCPULimitCoresFallsBackToQuotaPeriod(t *testing.T) {
	resources := container.Resources{CPUQuota: 50_000, CPUPeriod: 100_000}

	if got := cpuLimitCores(resources); got != 0.5 {
		t.Fatalf("cpuLimitCores() = %v, want 0.5", got)
	}
}

func TestFormatResourceLimits(t *testing.T) {
	if got := formatCPULimit(0); got != unlimitedResourceLabel {
		t.Fatalf("formatCPULimit(0) = %q, want %q", got, unlimitedResourceLabel)
	}
	if got := formatCPULimit(1.5); got != "1.50 cores" {
		t.Fatalf("formatCPULimit(1.5) = %q, want 1.50 cores", got)
	}
	if got := formatMemoryLimit(0); got != unlimitedResourceLabel {
		t.Fatalf("formatMemoryLimit(0) = %q, want %q", got, unlimitedResourceLabel)
	}
}

func TestNormalizeServerID(t *testing.T) {
	if got := normalizeServerID("Servidor PvP #2"); got != "servidor-pvp-2" {
		t.Fatalf("normalizeServerID() = %q, want servidor-pvp-2", got)
	}
}

func TestParseEnvLines(t *testing.T) {
	env, err := parseEnvLines("A=1\n# ignored\nB=two words\n")
	if err != nil {
		t.Fatalf("parseEnvLines() error = %v", err)
	}
	if strings.Join(env, ",") != "A=1,B=two words" {
		t.Fatalf("parseEnvLines() = %#v", env)
	}
}

func TestLoadRegistryFromPathsFallsBackToLegacyPath(t *testing.T) {
	dir := t.TempDir()
	primary := filepath.Join(dir, "luxview-games-servers.json")
	legacy := filepath.Join(dir, "games-companion-servers.json")
	content := `{"servers":[{"id":"legacy","template_id":"vrising","display_name":"Legacy V Rising","container_name":"luxview-vrising","game_port":"27015","protocol":"udp","image":"luxview/vrising","created_at":"now"}]}`

	if err := os.WriteFile(legacy, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	registry := loadRegistryFromPaths(primary, legacy)
	if len(registry.Servers) != 1 {
		t.Fatalf("loadRegistryFromPaths() loaded %d servers, want 1", len(registry.Servers))
	}
	if got := registry.Servers[0].ID; got != "legacy" {
		t.Fatalf("loadRegistryFromPaths() ID = %q, want legacy", got)
	}
}

func TestPortConfig(t *testing.T) {
	exposed, bindings, err := portConfig([]publishedPort{{ContainerPort: "27015", HostPort: "27017", Protocol: "udp"}})
	if err != nil {
		t.Fatalf("portConfig() error = %v", err)
	}
	port := nat.Port("27015/udp")
	if _, ok := exposed[port]; !ok {
		t.Fatalf("portConfig() missing exposed port %s", port)
	}
	if got := bindings[port][0].HostPort; got != "27017" {
		t.Fatalf("portConfig() HostPort = %q, want 27017", got)
	}
}

func TestParseResourceLimitInputs(t *testing.T) {
	cpu, err := parseCPUOrDefault("", 0.5)
	if err != nil || cpu != 0.5 {
		t.Fatalf("parseCPUOrDefault() = %v, %v, want 0.5 nil", cpu, err)
	}
	memory, err := parseMemoryLimitOrDefault("1.5g", defaultAppMemory)
	if err != nil || memory != 1610612736 {
		t.Fatalf("parseMemoryLimitOrDefault() = %d, %v, want 1610612736 nil", memory, err)
	}
}

func TestCPUPercentOfHost(t *testing.T) {
	if got := cpuPercentOfHost(1.5, 4); got != 37.5 {
		t.Fatalf("cpuPercentOfHost() = %v, want 37.5", got)
	}
}

func TestMemoryUsageBytesPrefersInactiveFile(t *testing.T) {
	stats := map[string]uint64{"inactive_file": 300, "cache": 200}

	if got := memoryUsageBytes(1000, stats); got != 700 {
		t.Fatalf("memoryUsageBytes() = %d, want 700", got)
	}
}

func TestSumNetworkBytes(t *testing.T) {
	networks := map[string]container.NetworkStats{
		"eth0": {RxBytes: 100, TxBytes: 200},
		"eth1": {RxBytes: 30, TxBytes: 40},
	}

	rx, tx := sumNetworkBytes(networks)
	if rx != 130 || tx != 240 {
		t.Fatalf("sumNetworkBytes() = %d, %d, want 130, 240", rx, tx)
	}
}

func TestFormatBytes(t *testing.T) {
	if got := formatBytes(1536); got != "1.5 KiB" {
		t.Fatalf("formatBytes() = %q, want 1.5 KiB", got)
	}
}

func TestQueryA2SHandlesChallengeResponse(t *testing.T) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket() error = %v", err)
	}
	defer conn.Close()

	go func() {
		buf := make([]byte, 1400)
		_, addr, err := conn.ReadFrom(buf)
		if err != nil {
			return
		}
		_, _ = conn.WriteTo([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x41, 0xED, 0xEE, 0x9E, 0xA4}, addr)
		_, addr, err = conn.ReadFrom(buf)
		if err != nil {
			return
		}
		_, _ = conn.WriteTo(a2sInfoResponse(3, 40), addr)
	}()

	info, err := queryA2S(conn.LocalAddr().String())
	if err != nil {
		t.Fatalf("queryA2S() error = %v", err)
	}
	if info.Players != 3 || info.MaxPlayers != 40 {
		t.Fatalf("queryA2S() = %+v, want 3/40", info)
	}
}

func a2sInfoResponse(players byte, maxPlayers byte) []byte {
	response := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x49, 0x11}
	for _, value := range []string{"Pseudo Gamers", "VRisingWorld", "V Rising", "e122dc48-7b14-4329-9e77-908d165afc06"} {
		response = append(response, value...)
		response = append(response, 0)
	}
	response = append(response, 0, 0, players, maxPlayers)
	return response
}
