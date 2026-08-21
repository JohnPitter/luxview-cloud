package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGameClientStorageResolvesGlobalReferenceWithoutAppService(t *testing.T) {
	root := t.TempDir()
	clientPath := filepath.Join(root, "metin2-assets", "client.zip")
	if err := os.MkdirAll(filepath.Dir(clientPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(clientPath, []byte("client"), 0644); err != nil {
		t.Fatal(err)
	}

	storage := NewClientStore(nil, nil, root, map[string]string{
		"metin2": clientPath,
	})
	got, err := storage.Resolve(context.Background(), uuid.New(), "metin2", "metin2-assets/client.zip")
	if err != nil {
		t.Fatal(err)
	}
	if got != clientPath {
		t.Fatalf("resolved path = %q, want %q", got, clientPath)
	}
}

func TestGameClientStorageRejectsGlobalPathTraversal(t *testing.T) {
	root := t.TempDir()
	if _, err := safeGlobalPath(root, "../outside.zip"); err == nil {
		t.Fatal("expected path traversal to be rejected")
	}
	if _, err := safeGlobalPath(root, filepath.Join(root, "client.zip")); err == nil {
		t.Fatal("expected absolute path to be rejected")
	}
}

func TestGameClientStorageListsZipReferences(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"metin2/client.zip": "client",
		"rakion/client.ZIP": "client",
		"readme.txt":        "ignore",
	}
	for name, contents := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}

	storage := NewClientStore(nil, nil, root, nil)
	options, err := storage.ListGlobalFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 2 {
		t.Fatalf("got %d options, want 2", len(options))
	}
	if options[0].Value != "metin2/client.zip" || options[1].Value != "rakion/client.ZIP" {
		t.Fatalf("unexpected options: %+v", options)
	}
}

func TestFileHashChangesWhenClientContentsChange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "tibia-assets", "client.zip")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("client-v1"), 0644); err != nil {
		t.Fatal(err)
	}
	storage := NewClientStore(nil, nil, root, nil)
	first, err := storage.FileHash(path)
	if err != nil {
		t.Fatal(err)
	}
	again, err := storage.FileHash(path)
	if err != nil {
		t.Fatal(err)
	}
	if first != again {
		t.Fatal("fingerprint should be stable for an unchanged file")
	}
	time.Sleep(time.Millisecond * 20)
	if err := os.WriteFile(path, []byte("client-v2"), 0644); err != nil {
		t.Fatal(err)
	}
	second, err := storage.FileHash(path)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("hash should change when the client zip is replaced")
	}
}
