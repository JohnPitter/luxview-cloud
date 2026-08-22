package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchPristonGameINIRewritesBPTConnectServer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "game.ini")
	src := "[ConnectServer]\nIP=189.46.228.170\nPort=30620\nClan= 189.46.228.170:30602\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := patchPristonGameINI(path, "187.77.227.65"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{"IP=187.77.227.65", "Port=10012", "Clan=187.77.227.65:10013"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
	if strings.Contains(text, "189.46.228.170") {
		t.Fatalf("BPT leftover: %q", text)
	}
}

func TestPatchPristonGameExeRewritesBPTConnectString(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Game.exe")
	src := append([]byte("hdr"), append(pristonBPTConnect, []byte("tail")...)...)
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := patchPristonGameExe(path, "187.77.227.65"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "189.46.228.170") {
		t.Fatal("BPT IP leftover in Game.exe")
	}
	if !strings.Contains(string(got), "187.77.227.65:10012") {
		t.Fatalf("LuxView connect missing: %q", got)
	}
}
