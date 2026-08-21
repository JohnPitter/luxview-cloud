package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/repository"
)

const ticketTTL = 2 * time.Minute

type accessTicket struct {
	userID uuid.UUID
	appID  uuid.UUID
	kind   string
	exp    time.Time
}

// AccessTickets issues short-lived opaque tickets so EventSource and native
// downloads do not put a JWT in the query string.
type AccessTickets struct {
	mu      sync.Mutex
	tickets map[string]accessTicket
}

func NewAccessTickets() *AccessTickets {
	return &AccessTickets{tickets: make(map[string]accessTicket)}
}

func (s *AccessTickets) Issue(userID, appID uuid.UUID, kind string) (string, time.Time, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", time.Time{}, err
	}
	id := hex.EncodeToString(raw[:])
	exp := time.Now().Add(ticketTTL)
	s.mu.Lock()
	s.gcLocked(time.Now())
	s.tickets[id] = accessTicket{userID: userID, appID: appID, kind: kind, exp: exp}
	s.mu.Unlock()
	return id, exp, nil
}

func (s *AccessTickets) Lookup(id, kind string, appID uuid.UUID) (uuid.UUID, bool) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gcLocked(now)
	t, ok := s.tickets[id]
	if !ok || t.kind != kind || t.appID != appID || now.After(t.exp) {
		return uuid.Nil, false
	}
	return t.userID, true
}

func (s *AccessTickets) gcLocked(now time.Time) {
	for id, t := range s.tickets {
		if now.After(t.exp) {
			delete(s.tickets, id)
		}
	}
}

// AuthOrTicket accepts a Bearer/cookie JWT, or a one-off ?ticket= for SSE/download.
func AuthOrTicket(jwtSecret string, users *repository.UserRepo, tickets *AccessTickets, kind string) func(http.Handler) http.Handler {
	auth := Auth(jwtSecret, users)
	return func(next http.Handler) http.Handler {
		ticketHandler := ticketOnly(users, tickets, kind, next)
		authHandler := auth(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("ticket") != "" {
				ticketHandler.ServeHTTP(w, r)
				return
			}
			authHandler.ServeHTTP(w, r)
		})
	}
}

func ticketOnly(users *repository.UserRepo, tickets *AccessTickets, kind string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appID, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid app ID")
			return
		}
		userID, ok := tickets.Lookup(r.URL.Query().Get("ticket"), kind, appID)
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		user, err := users.FindByID(r.Context(), userID)
		if err != nil || user == nil {
			writeJSONError(w, http.StatusUnauthorized, "user not found")
			return
		}
		ctx := r.Context()
		ctx = context.WithValue(ctx, UserIDKey, userID)
		ctx = context.WithValue(ctx, UserKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
