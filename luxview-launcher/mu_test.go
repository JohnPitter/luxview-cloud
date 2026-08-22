package main

import (
	"os"
	"path/filepath"
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

func TestEnsureMuEncTerrainStubsWorldsMissingObj(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "Data")
	if err := os.MkdirAll(filepath.Join(data, "World79"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(data, "World95"), 0o755); err != nil {
		t.Fatal(err)
	}
	donor := []byte("donor-obj")
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
	if string(got) != string(donor) {
		t.Fatalf("stub = %q", got)
	}
}
