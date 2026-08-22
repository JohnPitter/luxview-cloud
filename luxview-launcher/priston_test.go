package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchPristonLuncherINIRewritesGameServer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "luncher.ini")
	src := "[INITGAME]\ngameServerPORT=10012\ngameServerIP=127.0.0.1\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := patchPristonINI(path, "187.77.227.65", "LuxView"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{"gameServerIP=187.77.227.65", "gameServerPORT=10012"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
	if strings.Contains(text, "127.0.0.1") {
		t.Fatalf("localhost leftover: %q", text)
	}
}

func TestPatchPristonClientPrefersLuncherINI(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "luncher.ini"), []byte("[INITGAME]\ngameServerPORT=1\ngameServerIP=127.0.0.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ptReg.rgx"), []byte(`"Server1" "127.0.0.1"`+"\n"+`"ServerName" "SunnyBPT"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := patchPristonClient(dir, GameCard{ServerIP: "187.77.227.65", DisplayName: "LuxView"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "openpriston.launcher.ini")); !os.IsNotExist(err) {
		t.Fatal("4220 install should not grow an OpenPriston launcher.ini")
	}
	got, err := os.ReadFile(filepath.Join(dir, "luncher.ini"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "gameServerIP=187.77.227.65") {
		t.Fatalf("luncher.ini = %q", got)
	}
	reg, err := os.ReadFile(filepath.Join(dir, "ptReg.rgx"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(reg), `"Server1" "187.77.227.65"`) || !strings.Contains(string(reg), `"ServerName" "LuxView"`) {
		t.Fatalf("ptReg.rgx = %q", reg)
	}
}

func TestPristonExecutablePrefersSunnyBPT(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "game.exe"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SunnyBPT.exe"), []byte("native"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := pristonExecutable(dir)
	if !strings.HasSuffix(strings.ToLower(got), "sunnybpt.exe") {
		t.Fatalf("executable = %q", got)
	}
}

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
