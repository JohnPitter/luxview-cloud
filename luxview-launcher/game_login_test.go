package main

import (
	"os"
	"testing"
)

func TestSaveLoadGameLogin(t *testing.T) {
	id := "test-app-login"
	if err := saveGameLogin(id, "alice", "secret"); err != nil {
		t.Fatal(err)
	}
	path, err := gameLoginPath(id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	got, err := loadGameLogin(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.User != "alice" || got.Pass != "secret" {
		t.Fatalf("got %+v", got)
	}
}
