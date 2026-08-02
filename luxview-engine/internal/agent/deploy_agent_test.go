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

func TestResolveFreeModel(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		want      string
		wantError bool
	}{
		{name: "empty uses free router", want: DefaultFreeModel},
		{name: "free router", model: DefaultFreeModel, want: DefaultFreeModel},
		{name: "explicit free model", model: "openai/gpt-oss-20b:free", want: "openai/gpt-oss-20b:free"},
		{name: "paid model rejected", model: "anthropic/claude-sonnet-4", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveFreeModel(tt.model)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected an error for model %q", tt.model)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected model %q, got %q", tt.want, got)
			}
		})
	}
}
