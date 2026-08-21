package middleware

import (
	"context"
	"encoding/base64"
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
	UserIDKey   contextKey = "user_id"
	UserKey     contextKey = "user"
	PlayerIDKey contextKey = "player_id"
	PlayerKey   contextKey = "player"

	AudienceOperator = "luxview-operator"
	AudiencePlayer   = "luxview-player"
)

// JWTClaims holds the custom JWT claims.
type JWTClaims struct {
	UserID string         `json:"user_id"`
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
			Audience:  jwt.ClaimStrings{AudienceOperator},
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

type PlayerClaims struct {
	PlayerID string `json:"player_id"`
	jwt.RegisteredClaims
}

func GeneratePlayerJWT(playerID uuid.UUID, secret string) (string, error) {
	claims := PlayerClaims{
		PlayerID: playerID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "luxview-engine",
			Audience:  jwt.ClaimStrings{AudiencePlayer},
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func hasAudience(aud jwt.ClaimStrings, want string) bool {
	for _, a := range aud {
		if a == want {
			return true
		}
	}
	return false
}

// Auth is a middleware that validates JWT tokens.
func Auth(jwtSecret string, userRepo *repository.UserRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := logger.With("auth")

			tokenStr := extractToken(r)
			if tokenStr == "" {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
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
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if hasAudience(claims.Audience, AudiencePlayer) {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			userID, err := uuid.Parse(claims.UserID)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "invalid token claims")
				return
			}

			user, err := userRepo.FindByID(r.Context(), userID)
			if err != nil || user == nil {
				writeJSONError(w, http.StatusUnauthorized, "user not found")
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			ctx = context.WithValue(ctx, UserKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth validates a JWT token if present but does not reject requests without one.
// Useful for endpoints that serve both authenticated and anonymous users.
func OptionalAuth(jwtSecret string, userRepo *repository.UserRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractToken(r)
			if tokenStr == "" {
				next.ServeHTTP(w, r)
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
				next.ServeHTTP(w, r)
				return
			}

			userID, err := uuid.Parse(claims.UserID)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			user, err := userRepo.FindByID(r.Context(), userID)
			if err != nil || user == nil {
				next.ServeHTTP(w, r)
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
			writeJSONError(w, http.StatusForbidden, "forbidden")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// InternalAuth validates the internal API token. An empty token is fail-closed.
func InternalAuth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				writeJSONError(w, http.StatusServiceUnavailable, "internal auth not configured")
				return
			}
			if r.Header.Get("Authorization") != "Bearer "+token {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
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

func GetPlayerID(ctx context.Context) uuid.UUID {
	if id, ok := ctx.Value(PlayerIDKey).(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}

func GetPlayer(ctx context.Context) *model.PlayerAccount {
	if p, ok := ctx.Value(PlayerKey).(*model.PlayerAccount); ok {
		return p
	}
	return nil
}

func PlayerAuth(jwtSecret string, players *repository.PlayerRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractToken(r)
			if tokenStr == "" {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			claims := &PlayerClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			}, jwt.WithAudience(AudiencePlayer))
			if err != nil || !token.Valid {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			playerID, err := uuid.Parse(claims.PlayerID)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "invalid token claims")
				return
			}
			player, err := players.FindByID(r.Context(), playerID)
			if err != nil || player == nil {
				writeJSONError(w, http.StatusUnauthorized, "player not found")
				return
			}
			ctx := context.WithValue(r.Context(), PlayerIDKey, playerID)
			ctx = context.WithValue(ctx, PlayerKey, player)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractToken(r *http.Request) string {
	// Check Authorization header
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if strings.HasPrefix(auth, "Basic ") {
		encoded := strings.TrimSpace(strings.TrimPrefix(auth, "Basic "))
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err == nil {
			if _, password, ok := strings.Cut(string(decoded), ":"); ok && password != "" {
				return password
			}
		}
	}

	// Cookie (HttpOnly session). Query JWT is rejected — use a short-lived ticket.
	if cookie, err := r.Cookie("token"); err == nil {
		return cookie.Value
	}

	return ""
}
