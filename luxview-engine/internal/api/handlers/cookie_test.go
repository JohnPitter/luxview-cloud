package handlers

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCookieSecureUsesForwardedProto(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	if !cookieSecure(req) {
		t.Fatal("expected Secure behind Traefik HTTPS")
	}
}

func TestCookieSecurePlainHTTP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if cookieSecure(req) {
		t.Fatal("expected insecure cookie on plain HTTP")
	}
}

func TestCookieSecureTLS(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.TLS = &tls.ConnectionState{}
	if !cookieSecure(req) {
		t.Fatal("expected Secure on direct TLS")
	}
}
