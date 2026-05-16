package detector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNodeDockerfileUsesNode22AndPinnedPnpmForPnpmProjects(t *testing.T) {
	repoDir := t.TempDir()
	writeTestFile(t, repoDir, "package.json", `{"scripts":{"build":"vite build"},"dependencies":{"vite":"latest"}}`)
	writeTestFile(t, repoDir, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n")

	dockerfile := nodeDockerfile(Detection{Runtime: "nodejs", Framework: "vite", Port: 80}, repoDir)

	if !strings.Contains(dockerfile, "FROM node:22-alpine AS builder") {
		t.Fatalf("expected pnpm Dockerfile to use Node 22, got:\n%s", dockerfile)
	}
	if !strings.Contains(dockerfile, "corepack prepare pnpm@10 --activate") {
		t.Fatalf("expected pnpm Dockerfile to pin pnpm 10, got:\n%s", dockerfile)
	}
	if strings.Contains(dockerfile, "pnpm@latest") {
		t.Fatalf("expected pnpm Dockerfile not to use pnpm@latest, got:\n%s", dockerfile)
	}
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
