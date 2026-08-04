package service

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestPatchLegacyMetin2RootData(t *testing.T) {
	content := []byte("SERVER_IP = \"185.171.90.185\"\nPRIVATE_IP = \"192.168.2.100\"\nCH1_PORT = 13001\nMARKADDR = 13001\nAUTH_PORT = 11000\n")
	originalLength := len(content)
	patched, err := patchLegacyMetin2RootData(content, LegacyMetin2ClientOptions{
		ServerIP:  "187.77.227.65",
		AuthPort:  11000,
		WorldPort: 13001,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(patched)
	for _, expected := range []string{"0xBB.77.227.65", "PRIVATE_IP = \"187.77.227.65\"", "CH1_PORT = 13001", "MARKADDR = 13001", "AUTH_PORT = 11000"} {
		if !strings.Contains(text, expected) {
			t.Errorf("patched root.data missing %q: %s", expected, text)
		}
	}
	if len(patched) != originalLength {
		t.Fatalf("root.data length changed from %d to %d", originalLength, len(patched))
	}
}

func TestPatchLegacyMetin2RootDataSupportsS1llClientLayout(t *testing.T) {
	content := []byte("SERVER_IP = \"000.000.000.000\"\nCH1_PORT = 13101\nMARKADDR = 13101\nAUTH_PORT = 11100\n")
	originalLength := len(content)
	patched, err := patchLegacyMetin2RootData(content, LegacyMetin2ClientOptions{
		ServerIP:  "187.77.227.65",
		AuthPort:  11000,
		WorldPort: 13001,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(patched)
	for _, expected := range []string{
		"SERVER_IP = \"187.0x4D.227.65\"",
		"CH1_PORT = 13001",
		"MARKADDR = 13001",
		"AUTH_PORT = 11000",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("patched s1ll root.data missing %q: %s", expected, text)
		}
	}
	if len(patched) != originalLength {
		t.Fatalf("root.data length changed from %d to %d", originalLength, len(patched))
	}
}

func TestWriteLegacyMetin2ClientZipPatchesRootAndLocale(t *testing.T) {
	base := newZip(t, map[string]string{
		"Metin2FullClient/pack/root.data":       "SERVER_IP = \"000.000.000.000\"\nCH1_PORT = 13101\nMARKADDR = 13101\nAUTH_PORT = 11100\n",
		"Metin2FullClient/locale.cfg":           "1252 tr\n",
		"Metin2FullClient/Metin2Distribute.exe": "client",
	})

	var output bytes.Buffer
	err := WriteLegacyMetin2ClientZip(bytes.NewReader(base), int64(len(base)), &output, LegacyMetin2ClientOptions{
		ServerIP:  "187.77.227.65",
		AuthPort:  11000,
		WorldPort: 13001,
	})
	if err != nil {
		t.Fatal(err)
	}
	files := readZip(t, output.Bytes())
	if !strings.Contains(files["Metin2FullClient/pack/root.data"], "187.0x4D.227.65") {
		t.Error("root.data was not patched with the server IP")
	}
	if files["Metin2FullClient/locale.cfg"] != "1252 pt\n" {
		t.Errorf("locale.cfg = %q, want pt-BR", files["Metin2FullClient/locale.cfg"])
	}
}

func newZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func readZip(t *testing.T, data []byte) map[string]string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string]string, len(reader.File))
	for _, file := range reader.File {
		entry, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(entry)
		entry.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[file.Name] = string(content)
	}
	return files
}
