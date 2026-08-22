package service

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestWritePristonClientZipPatchesRegistryAndINI(t *testing.T) {
	var base bytes.Buffer
	writer := zip.NewWriter(&base)
	reg, err := writer.Create("ptReg.rgx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(reg, `"Server1" "127.0.0.1"`+"\n"+`"ServerName" "BPT"`+"\n"); err != nil {
		t.Fatal(err)
	}
	ini, err := writer.Create("openpriston.launcher.ini")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(ini, "ServerAddress=127.0.0.1\nServerName=BPT\nServerPort=10012\n"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := WritePristonClientZip(bytes.NewReader(base.Bytes()), int64(base.Len()), &out, PristonClientOptions{
		ServerName: "LuxView",
		ServerIP:   "187.77.227.65",
		GamePort:   10012,
	}); err != nil {
		t.Fatal(err)
	}

	reader, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatal(err)
	}
	foundReg := false
	foundINI := false
	for _, file := range reader.File {
		body, err := readZipFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		switch {
		case strings.EqualFold(file.Name, "ptReg.rgx"):
			foundReg = true
			if !strings.Contains(text, `"Server1" "187.77.227.65"`) || !strings.Contains(text, `"ServerName" "LuxView"`) {
				t.Fatalf("ptReg.rgx = %q", text)
			}
			if strings.Contains(text, "BPT") {
				t.Fatalf("BPT leftover in ptReg.rgx: %q", text)
			}
		case strings.EqualFold(file.Name, "openpriston.launcher.ini"):
			foundINI = true
			if !strings.Contains(text, "ServerAddress=187.77.227.65") || !strings.Contains(text, "ServerName=LuxView") {
				t.Fatalf("ini = %q", text)
			}
		}
	}
	if !foundReg || !foundINI {
		t.Fatalf("missing patched files reg=%v ini=%v", foundReg, foundINI)
	}
}
