package main

import "testing"

func TestAllowedUpdateURLAcceptsOfficialRelease(t *testing.T) {
	url := "https://github.com/JohnPitter/luxview-cloud/releases/download/launcher-v1.62/luxview-launcher.exe"
	if err := allowedUpdateURL(url); err != nil {
		t.Fatalf("official release rejected: %v", err)
	}
}

func TestAllowedUpdateURLRejectsArbitraryHost(t *testing.T) {
	if err := allowedUpdateURL("https://evil.example/luxview-launcher.exe"); err == nil {
		t.Fatal("expected arbitrary host to be rejected")
	}
}

func TestAllowedUpdateURLRejectsHTTP(t *testing.T) {
	if err := allowedUpdateURL("http://github.com/JohnPitter/luxview-cloud/releases/download/x/y"); err == nil {
		t.Fatal("expected http to be rejected")
	}
}

func TestAllowedUpdateURLRejectsOtherRepo(t *testing.T) {
	if err := allowedUpdateURL("https://github.com/other/repo/releases/download/x/y"); err == nil {
		t.Fatal("expected other repo to be rejected")
	}
}
