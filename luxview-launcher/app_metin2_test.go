package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMetin2LaunchSpecUsesLegacyClientLayout(t *testing.T) {
	spec, ok := launchSpecs["metin2"]
	if !ok {
		t.Fatal("metin2 launch spec is not registered")
	}
	if spec.clientDir != "Metin2FullClient" || spec.gameExe != "Metin2Distribute.exe" {
		t.Fatalf("legacy client layout = %q/%q", spec.clientDir, spec.gameExe)
	}
	if spec.loginPath != "" || spec.registerPath != "" {
		t.Fatalf("Metin2 must authenticate inside the game, got login=%q register=%q", spec.loginPath, spec.registerPath)
	}
}

func TestMetin2InstallRequiresRuntimeFiles(t *testing.T) {
	spec := launchSpecs["metin2"]
	root := t.TempDir()
	clientDir := filepath.Join(root, spec.clientDir)
	for _, file := range requiredClientFiles("metin2", spec) {
		path := filepath.Join(clientDir, file)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("client"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if !clientFilesReady(root, "metin2", spec) {
		t.Fatal("complete Metin2 client was not recognized")
	}
	if err := os.Remove(filepath.Join(clientDir, "python27.dll")); err != nil {
		t.Fatal(err)
	}
	if clientFilesReady(root, "metin2", spec) {
		t.Fatal("Metin2 client without python27.dll was recognized as ready")
	}
}
