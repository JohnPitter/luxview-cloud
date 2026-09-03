package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMuEncodeHostAndPortMatchOpenMULauncher(t *testing.T) {
	ip := "187.77.227.65"
	got := []rune(muEncodeHost(ip))
	if len(got) != len(ip) {
		t.Fatalf("encoded host length %d", len(got))
	}
	if got[0] != rune('1'+12) || got[1] != rune('8'+7) || got[2] != rune('7'+3) || got[3] != rune('.'+0x13) {
		t.Fatalf("encoded host prefix %q", string(got[:4]))
	}
	if muEncodePort(ip, 44405) != 44402 {
		t.Fatalf("encoded port %d", muEncodePort(ip, 44405))
	}
}

func TestPatchMuServerInfoRewritesIGCConnection(t *testing.T) {
	raw := []byte("; header\r\n[Connection]\r\nIP=\"192.168.0.168\"\r\nPort=44405\r\nChatPort=56980\r\n\r\n[Main]\r\nVersion=\"1.05.25\"\r\nSerial=\"PoweredByIGCN800\"\r\n")
	got := string(patchMuServerInfo(raw, "187.77.227.65", 44405, 55980))
	for _, want := range []string{`IP="187.77.227.65"`, "Port=44405", "ChatPort=55980", `Serial="PoweredByIGCN800"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "192.168.0.168") || strings.Contains(got, "ChatPort=56980") {
		t.Fatalf("old connection still present: %q", got)
	}
}

func TestPatchMuPackedIPSameLength(t *testing.T) {
	raw := []byte("pre\x00192.168.0.168\x00post")
	got := patchMuPackedIP(raw, "187.77.227.65")
	if !bytes.Contains(got, []byte("187.77.227.65")) {
		t.Fatalf("missing vps ip: %q", got)
	}
	if bytes.Contains(got, []byte("192.168.0.168")) {
		t.Fatal("old lan ip still present")
	}
}

func TestPatchMuServerInfoFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Data", "Local")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "ServerInfo.bmd")
	if err := os.WriteFile(path, []byte("IP=\"192.168.0.168\"\nPort=1\nChatPort=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := patchMuServerInfoFiles(root, "187.77.227.65", 44405, 55980); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `IP="187.77.227.65"`) {
		t.Fatalf("got %q", got)
	}
}

func TestMuMapCryptRoundTrip(t *testing.T) {
	plain := []byte{0x00, 0x4f, 0x02, 0x00, 0xaa, 0xbb, 0xcc, 0xdd}
	if got := muDecryptMapFile(muEncryptMapFile(plain)); !bytesEqual(got, plain) {
		t.Fatalf("roundtrip %x", got)
	}
}

func TestEnsureMuEncTerrainRetagsWorldID(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "Data")
	if err := os.MkdirAll(filepath.Join(data, "World79"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(data, "World95"), 0o755); err != nil {
		t.Fatal(err)
	}
	plain := []byte{0x00, 0x4f, 0x00, 0x00}
	donor := muEncryptMapFile(plain)
	if err := os.WriteFile(filepath.Join(data, "World79", "EncTerrain79.obj"), donor, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "World95", "EncTerrain95.map"), []byte("map"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureMuEncTerrain(root); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(data, "World95", "EncTerrain95.obj"))
	if err != nil {
		t.Fatal(err)
	}
	id := muDecryptMapFile(got)
	if len(id) < 2 || int(id[0])<<8|int(id[1]) != 95 {
		t.Fatalf("world id = %x", id)
	}
}

func TestEnsureMuEncTerrainRetagsExistingStub(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "Data")
	if err := os.MkdirAll(filepath.Join(data, "World95"), 0o755); err != nil {
		t.Fatal(err)
	}
	stub := muEncryptMapFile([]byte{0x00, 0x4f, 0x00, 0x00})
	if err := os.WriteFile(filepath.Join(data, "World95", "EncTerrain95.obj"), stub, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "World95", "EncTerrain95.map"), []byte("map"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureMuEncTerrain(root); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(data, "World95", "EncTerrain95.obj"))
	if err != nil {
		t.Fatal(err)
	}
	id := muDecryptMapFile(got)
	if len(id) < 2 || int(id[0])<<8|int(id[1]) != 95 {
		t.Fatalf("world id = %x", id)
	}
}

func TestParseMuServerEntriesGroupsHierarchy(t *testing.T) {
	// Live layout: S6=0, 99d PvP=20, 99d PvE=21, S2=40.
	body := make([]byte, 16)
	ids := []uint16{0, 20, 21, 40}
	loads := []byte{0, 12, 0, 40}
	for i, id := range ids {
		body[i*4] = byte(id)
		body[i*4+1] = byte(id >> 8)
		body[i*4+2] = loads[i]
	}
	got := parseMuServerEntries(body, 4)
	if len(got) != 4 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].ID != 20 || got[0].Name != "99d" || got[0].Difficulty != "Hard" || got[0].Mode != "PvP" || got[0].Channel != 1 {
		t.Fatalf("first = %+v", got[0])
	}
	if got[1].ID != 21 || got[1].Name != "99d" || got[1].Mode != "PvE" || got[1].Channel != 2 {
		t.Fatalf("pve = %+v", got[1])
	}
	if got[2].ID != 40 || got[2].Name != "Season 2" || got[2].Difficulty != "Medium" {
		t.Fatalf("s2 = %+v", got[2])
	}
	if got[3].ID != 0 || got[3].Name != "Season 6 Pt 3" || got[3].Difficulty != "Easy" {
		t.Fatalf("s6 = %+v", got[3])
	}
}

func TestApplyMuServerCatalogOverrides(t *testing.T) {
	servers := []MuServerInfo{muAnnotate(20, 0), muAnnotate(21, 5)}
	applyMuServerCatalog(servers, []MuServerGroup{{
		Group: 1, Name: "Lux 99d", Difficulty: "Medium",
		Channels: []MuChannelMeta{{ID: 21, Mode: "PvP"}},
	}})
	if servers[0].Name != "Lux 99d" || servers[0].Difficulty != "Medium" {
		t.Fatalf("group overlay = %+v", servers[0])
	}
	if servers[1].Mode != "PvP" {
		t.Fatalf("mode overlay = %+v", servers[1])
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
