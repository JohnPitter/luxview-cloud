package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBaseURLPrefersEnvOverLdflags(t *testing.T) {
	t.Setenv("LUXVIEW_BASE_URL", "https://staging.example/")
	luxviewBaseURL = "https://from-ldflags.example"
	defer func() { luxviewBaseURL = "" }()

	if got := baseURL(); got != "https://staging.example" {
		t.Fatalf("baseURL() = %q", got)
	}
}

func TestBaseURLUsesLdflagsWhenEnvEmpty(t *testing.T) {
	t.Setenv("LUXVIEW_BASE_URL", "")
	luxviewBaseURL = "https://from-ldflags.example/"
	defer func() { luxviewBaseURL = "" }()

	if got := baseURL(); got != "https://from-ldflags.example" {
		t.Fatalf("baseURL() = %q", got)
	}
}

func TestPlatformOriginRequiresValue(t *testing.T) {
	t.Setenv("LUXVIEW_BASE_URL", "")
	luxviewBaseURL = ""
	if _, err := platformOrigin(); err == nil {
		t.Fatal("expected error when origin is unset")
	}
}

func TestApplyDotEnvFileDoesNotOverrideOS(t *testing.T) {
	t.Setenv("LUXVIEW_BASE_URL", "https://already-set.example")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("LUXVIEW_BASE_URL=https://from-file.example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	applyDotEnvFile(path)
	if got := os.Getenv("LUXVIEW_BASE_URL"); got != "https://already-set.example" {
		t.Fatalf("env = %q", got)
	}
}
