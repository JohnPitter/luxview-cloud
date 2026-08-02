package middleware

import (
	"encoding/base64"
	"net/http/httptest"
	"testing"
)

func TestExtractTokenAcceptsGitBasicAuthentication(t *testing.T) {
	req := httptest.NewRequest("GET", "/git/alice/repo.git/info/refs", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("alice:git-token")))

	if got := extractToken(req); got != "git-token" {
		t.Fatalf("extractToken() = %q, want git-token", got)
	}
}

func TestExtractTokenIgnoresMalformedGitBasicAuthentication(t *testing.T) {
	req := httptest.NewRequest("GET", "/git/alice/repo.git/info/refs", nil)
	req.Header.Set("Authorization", "Basic not-base64")

	if got := extractToken(req); got != "" {
		t.Fatalf("extractToken() = %q, want empty token", got)
	}
}
