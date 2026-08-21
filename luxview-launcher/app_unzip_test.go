package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestZipEntryTargetRejectsTraversal(t *testing.T) {
	dest := t.TempDir()
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zipEntryTarget(dest, destAbs, `..\outside.bin`); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}

func TestZipEntryTargetAcceptsWindowsSeparators(t *testing.T) {
	dest := t.TempDir()
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		t.Fatal(err)
	}
	got, err := zipEntryTarget(dest, destAbs, `client\Bin\load.bin`)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dest, "client", "Bin", "load.bin")
	if got != want {
		t.Fatalf("target = %q, want %q", got, want)
	}
}

func TestUnzipOverwritesReadOnlyFile(t *testing.T) {
	dest := t.TempDir()
	target := filepath.Join(dest, "client", "Bin", "load.bin")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0o444); err != nil {
		t.Fatal(err)
	}

	zipPath := filepath.Join(t.TempDir(), "client.zip")
	if err := writeTestZip(zipPath, "client/Bin/load.bin", []byte("new")); err != nil {
		t.Fatal(err)
	}
	if err := unzip(zipPath, dest, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("extracted content = %q, want new", got)
	}
}

func writeTestZip(path, name string, content []byte) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	w := zip.NewWriter(file)
	entry, err := w.Create(name)
	if err != nil {
		return err
	}
	if _, err := entry.Write(content); err != nil {
		return err
	}
	return w.Close()
}
