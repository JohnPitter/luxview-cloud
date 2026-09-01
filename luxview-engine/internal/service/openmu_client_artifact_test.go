package service

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func TestBuildOpenMULauncherConfigIncludesServerConnection(t *testing.T) {
	config := BuildOpenMULauncherConfig("Aida MU", "187.77.227.65", 44405)

	assertContains(t, config, "<MainExePath>main.exe</MainExePath>")
	assertContains(t, config, "<Description>Aida MU</Description>")
	assertContains(t, config, "<Address>187.77.227.65</Address>")
	assertContains(t, config, "<Port>44405</Port>")
}

func TestWriteOpenMUClientZipReplacesLauncherConfig(t *testing.T) {
	var base bytes.Buffer
	baseZip := zip.NewWriter(&base)
	addZipEntry(t, baseZip, "main.exe", "fake-main")
	addZipEntry(t, baseZip, "launcher.config", "old-config")
	if err := baseZip.Close(); err != nil {
		t.Fatalf("close base zip: %v", err)
	}

	var out bytes.Buffer
	if err := WriteOpenMUClientZip(bytes.NewReader(base.Bytes()), int64(base.Len()), &out, OpenMUClientOptions{
		ServerName: "Aida MU",
		ServerIP:   "187.77.227.65",
		GamePort:   44405,
	}); err != nil {
		t.Fatalf("write client zip: %v", err)
	}

	got := readZipEntries(t, out.Bytes())
	if got["main.exe"] != "fake-main" {
		t.Fatalf("main.exe content = %q", got["main.exe"])
	}
	if strings.Contains(got["launcher.config"], "old-config") {
		t.Fatalf("launcher.config was not replaced: %q", got["launcher.config"])
	}
	assertContains(t, got["launcher.config"], "<Description>Aida MU</Description>")
	assertContains(t, got["launcher.config"], "<Address>187.77.227.65</Address>")
	assertContains(t, got["launcher.config"], "<Port>44405</Port>")
}

func TestWriteOpenMUClientZipPatchesIGCServerInfo(t *testing.T) {
	var base bytes.Buffer
	baseZip := zip.NewWriter(&base)
	addZipEntry(t, baseZip, "Data/Local/ServerInfo.bmd", "IP=\"192.168.0.168\"\nPort=44405\nChatPort=56980\nSerial=\"PoweredByIGCN800\"\n")
	addZipEntry(t, baseZip, "main.exe", "fake-main")
	if err := baseZip.Close(); err != nil {
		t.Fatalf("close base zip: %v", err)
	}

	var out bytes.Buffer
	if err := WriteOpenMUClientZip(bytes.NewReader(base.Bytes()), int64(base.Len()), &out, OpenMUClientOptions{
		ServerName: "Aida MU",
		ServerIP:   "187.77.227.65",
		GamePort:   44405,
	}); err != nil {
		t.Fatalf("write client zip: %v", err)
	}
	got := readZipEntries(t, out.Bytes())["Data/Local/ServerInfo.bmd"]
	assertContains(t, got, `IP="187.77.227.65"`)
	assertContains(t, got, "ChatPort=55980")
	assertContains(t, got, `Serial="PoweredByIGCN800"`)
	if strings.Contains(got, "192.168.0.168") {
		t.Fatalf("old lan ip still present: %q", got)
	}
}

func TestWriteOpenMUClientZipPatchesPackedConnectIP(t *testing.T) {
	var base bytes.Buffer
	baseZip := zip.NewWriter(&base)
	addZipEntry(t, baseZip, "IGC.dll", "x\x00192.168.0.168\x00y")
	addZipEntry(t, baseZip, "main.exe", "fake-main")
	if err := baseZip.Close(); err != nil {
		t.Fatalf("close base zip: %v", err)
	}
	var out bytes.Buffer
	if err := WriteOpenMUClientZip(bytes.NewReader(base.Bytes()), int64(base.Len()), &out, OpenMUClientOptions{
		ServerIP: "187.77.227.65",
		GamePort: 44405,
	}); err != nil {
		t.Fatalf("write client zip: %v", err)
	}
	got := readZipEntries(t, out.Bytes())
	assertContains(t, got["IGC.dll"], "187.77.227.65")
	if strings.Contains(got["IGC.dll"], "192.168.0.168") {
		t.Fatalf("old lan ip in IGC.dll: %q", got["IGC.dll"])
	}
}

func TestWriteOpenMUClientPatchOverlaysMainExeOnly(t *testing.T) {
	var base bytes.Buffer
	baseZip := zip.NewWriter(&base)
	addZipEntry(t, baseZip, "main.exe", "new-main")
	addZipEntry(t, baseZip, "Data/Local/ServerInfo.bmd", "IP=\"192.168.0.168\"\nPort=1\nChatPort=2\n")
	addZipEntry(t, baseZip, "Data/World1/player.bmd", "keep-me")
	if err := baseZip.Close(); err != nil {
		t.Fatalf("close base zip: %v", err)
	}

	var out bytes.Buffer
	if err := WriteOpenMUClientPatch(bytes.NewReader(base.Bytes()), int64(base.Len()), &out, OpenMUClientOptions{
		ServerName: "LuxView",
		ServerIP:   "187.77.227.65",
		GamePort:   44405,
	}); err != nil {
		t.Fatalf("patch: %v", err)
	}
	got := readZipEntries(t, out.Bytes())
	if got["main.exe"] != "new-main" {
		t.Fatalf("patch must overlay main.exe, got %q", got["main.exe"])
	}
	if _, ok := got["Data/World1/player.bmd"]; ok {
		t.Fatal("patch must not re-ship the full Data/ tree")
	}
	assertContains(t, got["launcher.config"], "<Address>187.77.227.65</Address>")
	assertContains(t, got["Data/Local/ServerInfo.bmd"], `IP="187.77.227.65"`)
}

func TestWriteOpenMUClientPatchRetagsEncTerrainWorldID(t *testing.T) {
	donor := muEncryptMapFile([]byte{0x00, 0x4f, 0x00, 0x00})
	var base bytes.Buffer
	baseZip := zip.NewWriter(&base)
	addZipEntry(t, baseZip, "Data/World79/EncTerrain79.obj", string(donor))
	addZipEntry(t, baseZip, "Data/World95/EncTerrain95.map", "map-95")
	if err := baseZip.Close(); err != nil {
		t.Fatalf("close base zip: %v", err)
	}

	var out bytes.Buffer
	if err := WriteOpenMUClientPatch(bytes.NewReader(base.Bytes()), int64(base.Len()), &out, OpenMUClientOptions{
		ServerName: "Aida MU",
		ServerIP:   "187.77.227.65",
		GamePort:   44405,
	}); err != nil {
		t.Fatalf("patch: %v", err)
	}
	got := readZipEntries(t, out.Bytes())
	assertContains(t, got["launcher.config"], "<Address>187.77.227.65</Address>")
	plain := muDecryptMapFile([]byte(got["Data/World95/EncTerrain95.obj"]))
	if len(plain) < 2 || int(plain[0])<<8|int(plain[1]) != 95 {
		t.Fatalf("World95 world id = %x", plain)
	}
}

func addZipEntry(t *testing.T, zw *zip.Writer, name string, content string) {
	t.Helper()
	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("create zip entry %s: %v", name, err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatalf("write zip entry %s: %v", name, err)
	}
}

func readZipEntries(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open generated zip: %v", err)
	}
	entries := make(map[string]string, len(zr.File))
	for _, file := range zr.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", file.Name, err)
		}
		var content bytes.Buffer
		if _, err := content.ReadFrom(rc); err != nil {
			rc.Close()
			t.Fatalf("read zip entry %s: %v", file.Name, err)
		}
		rc.Close()
		entries[file.Name] = content.String()
	}
	return entries
}

func assertContains(t *testing.T, value string, expected string) {
	t.Helper()
	if !strings.Contains(value, expected) {
		t.Fatalf("expected %q to contain %q", value, expected)
	}
}
