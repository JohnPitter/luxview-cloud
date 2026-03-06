package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/logger"
)

type contextKey string

const (
	UserIDKey contextKey = "user_id"
	UserKey   contextKey = "user"
)

// JWTClaims holds the custom JWT claims.
type JWTClaims struct {
	UserID string    `json:"user_id"`
	Role   model.UserRole `json:"role"`
	jwt.RegisteredClaims
}

// GenerateJWT creates a new JWT token for the given user.
func GenerateJWT(userID uuid.UUID, role model.UserRole, secret string) (string, error) {
	claims := JWTClaims{
		UserID: userID.String(),
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "luxview-engine",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// Auth is a middleware that validates JWT tokens.
func Auth(jwtSecret string, userRepo *repository.UserRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := logger.With("auth")

			tokenStr := extractToken(r)
			if tokenStr == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			claims := &JWTClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			})

			if err != nil || !token.Valid {
				log.Debug().Err(err).Msg("invalid token")
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			userID, err := uuid.Parse(claims.UserID)
			if err != nil {
				http.Error(w, `{"error":"invalid token claims"}`, http.StatusUnauthorized)
				return
			}

			user, err := userRepo.FindByID(r.Context(), userID)
			if err != nil || user == nil {
				http.Error(w, `{"error":"user not found"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			ctx = context.WithValue(ctx, UserKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AdminOnly restricts access to admin users.
func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r.Context())
		if user == nil || user.Role != model.RoleAdmin {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// InternalAuth validates the internal API token.
func InternalAuth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				// No internal token configured, allow all
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader != "Bearer "+token {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GetUserID extracts the user ID from the request context.
func GetUserID(ctx context.Context) uuid.UUID {
	if id, ok := ctx.Value(UserIDKey).(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}

// GetUser extracts the user from the request context.
func GetUser(ctx context.Context) *model.User {
	if u, ok := ctx.Value(UserKey).(*model.User); ok {
		return u
	}
	return nil
}

func extractToken(r *http.Request) string {
	// Check Authorization header
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	// Check cookie
	if cookie, err := r.Cookie("token"); err == nil {
		return cookie.Value
	}

	// Check query param (for WebSocket connections)
	if token := r.URL.Query().Get("token"); token != "" {
		return token
	}

	return ""
}
