package middleware

import (
	"testing"

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
