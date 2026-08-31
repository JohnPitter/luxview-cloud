package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAccessTicketsRoundTrip(t *testing.T) {
	s := NewAccessTickets()
	user := uuid.New()
	app := uuid.New()
	id, _, err := s.Issue(user, app, "logs")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s.Lookup(id, "logs", app)
	if !ok || got != user {
		t.Fatalf("lookup = %v %v, want user", got, ok)
	}
	if _, ok := s.Lookup(id, "download", app); ok {
		t.Fatal("wrong kind must fail")
	}
	if _, ok := s.Lookup(id, "logs", uuid.New()); ok {
		t.Fatal("wrong app must fail")
	}
}

func TestAccessTicketsAdminPanelTTL(t *testing.T) {
	s := NewAccessTickets()
	user := uuid.New()
	app := uuid.New()
	id, exp, err := s.Issue(user, app, TicketKindAdminPanel)
	if err != nil {
		t.Fatal(err)
	}
	if time.Until(exp) < 7*time.Hour {
		t.Fatalf("admin-panel ticket ttl too short: %v", time.Until(exp))
	}
	got, ok := s.Lookup(id, TicketKindAdminPanel, app)
	if !ok || got != user {
		t.Fatalf("lookup = %v %v", got, ok)
	}
}

func TestTicketIDFromCookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/apps/x/game-admin/", nil)
	req.AddCookie(&http.Cookie{Name: AdminPanelCookie, Value: "abc"})
	if got := ticketID(req, TicketKindAdminPanel); got != "abc" {
		t.Fatalf("ticketID cookie = %q", got)
	}
	if got := ticketID(req, TicketKindLogs); got != "" {
		t.Fatalf("logs kind should ignore admin cookie, got %q", got)
	}
}
