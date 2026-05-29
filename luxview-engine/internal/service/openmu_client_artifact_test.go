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
