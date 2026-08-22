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
	gameIni, err := writer.Create("game.ini")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(gameIni, "[ConnectServer]\nIP=189.46.228.170\nPort=30620\nClan= 189.46.228.170:30602\n"); err != nil {
		t.Fatal(err)
	}
	exe, err := writer.Create("Game.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exe.Write([]byte("hdr" + "189.46.228.170:30303" + "tail")); err != nil {
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
	foundGameINI := false
	foundExe := false
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
		case strings.EqualFold(file.Name, "game.ini"):
			foundGameINI = true
			if !strings.Contains(text, "IP=187.77.227.65") || !strings.Contains(text, "Port=10012") || !strings.Contains(text, "Clan=187.77.227.65:10013") {
				t.Fatalf("game.ini = %q", text)
			}
			if strings.Contains(text, "189.46.228.170") {
				t.Fatalf("BPT leftover in game.ini: %q", text)
			}
		case strings.EqualFold(file.Name, "Game.exe"):
			foundExe = true
			if !bytes.Contains(body, []byte("187.77.227.65:10012")) || bytes.Contains(body, []byte("189.46.228.170")) {
				t.Fatalf("Game.exe still has BPT connect string")
			}
		}
	}
	if !foundReg || !foundINI || !foundGameINI || !foundExe {
		t.Fatalf("missing patched files reg=%v ini=%v game.ini=%v exe=%v", foundReg, foundINI, foundGameINI, foundExe)
	}
}
