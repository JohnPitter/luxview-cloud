package middleware

import (
	"encoding/base64"
	"net/http"
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

func TestExtractTokenIgnoresQueryJWT(t *testing.T) {
	req := httptest.NewRequest("GET", "/apps/x/logs/stream?token=secret-jwt", nil)
	if got := extractToken(req); got != "" {
		t.Fatalf("extractToken() = %q, want empty (query JWT is not accepted)", got)
	}
}

func TestInternalAuthRejectsEmptyToken(t *testing.T) {
	h := InternalAuth("")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("should not reach handler")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/internal/traefik-config", nil))
	if rec.Code != 503 {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}
