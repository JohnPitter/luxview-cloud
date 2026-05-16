package agent

import (
	"strings"
	"testing"
)

func TestSanitizeDockerfileNormalizesPnpmNodeCompatibility(t *testing.T) {
	input := `FROM node:20-alpine
WORKDIR /app
RUN corepack enable && corepack prepare pnpm@latest --activate
RUN pnpm install --frozen-lockfile`

	dockerfile := sanitizeDockerfile(input)

	if !strings.Contains(dockerfile, "FROM node:22-alpine") {
		t.Fatalf("expected pnpm Dockerfile to use Node 22, got:\n%s", dockerfile)
	}
	if !strings.Contains(dockerfile, "corepack prepare pnpm@10 --activate") {
		t.Fatalf("expected pnpm Dockerfile to pin pnpm 10, got:\n%s", dockerfile)
	}
	if strings.Contains(dockerfile, "pnpm@latest") || strings.Contains(dockerfile, "node:20-alpine") {
		t.Fatalf("expected legacy pnpm Dockerfile values to be removed, got:\n%s", dockerfile)
	}
}
