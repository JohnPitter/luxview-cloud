package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/config"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/crypto"
	"github.com/luxview/engine/pkg/logger"
)

type AuthHandler struct {
	cfg           *config.Config
	userRepo      *repository.UserRepo
	settingsRepo  *repository.SettingsRepo
	github        *service.GitHubClient
	encryptionKey []byte
	auditSvc      *service.AuditService
}

func NewAuthHandler(cfg *config.Config, userRepo *repository.UserRepo, settingsRepo *repository.SettingsRepo, encryptionKey []byte, auditSvc *service.AuditService) *AuthHandler {
	return &AuthHandler{
		cfg:           cfg,
		userRepo:      userRepo,
		settingsRepo:  settingsRepo,
		github:        service.NewGitHubClient(),
		encryptionKey: encryptionKey,
		auditSvc:      auditSvc,
	}
}

// GitHubLogin redirects to GitHub OAuth authorization page.
// Always allows the OAuth flow — admin check happens in the callback after authentication.
func (h *AuthHandler) GitHubLogin(w http.ResponseWriter, r *http.Request) {
	url := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&scope=repo,user:email&redirect_uri=%s/api/auth/github/callback",
		h.cfg.GitHubClientID,
		h.cfg.BaseURL,
	)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// GitHubCallback handles the OAuth callback, exchanges code for token, creates/updates user.
func (h *AuthHandler) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	log := logger.With("auth")
	ctx := r.Context()

	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing code parameter")
		return
	}

	// Exchange code for token
	tokenResp, err := h.github.ExchangeCode(ctx, h.cfg.GitHubClientID, h.cfg.GitHubClientSecret, code)
	if err != nil {
		log.Error().Err(err).Msg("failed to exchange code")
		writeError(w, http.StatusBadGateway, "failed to authenticate with GitHub")
		return
	}

	// Get user info
	ghUser, err := h.github.GetUser(ctx, tokenResp.AccessToken)
	if err != nil {
		log.Error().Err(err).Msg("failed to get GitHub user")
		writeError(w, http.StatusBadGateway, "failed to get user info from GitHub")
		return
	}

	email := ghUser.Email
	if email == "" {
		email, _ = h.github.GetUserEmail(ctx, tokenResp.AccessToken)
	}

	// Encrypt the GitHub token
	encryptedToken, err := crypto.Encrypt(tokenResp.AccessToken, h.encryptionKey)
	if err != nil {
		log.Error().Err(err).Msg("failed to encrypt token")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Upsert user
	user := &model.User{
		GitHubID:    ghUser.ID,
		Username:    ghUser.Login,
		Email:       email,
		AvatarURL:   ghUser.AvatarURL,
		GitHubToken: encryptedToken,
		Role:        model.RoleUser,
	}

	if err := h.userRepo.Upsert(ctx, user); err != nil {
		log.Error().Err(err).Msg("failed to upsert user")
		writeError(w, http.StatusInternalServerError, "failed to save user")
		return
	}

	// If maintenance mode is active, only admins can log in
	if val, err := h.settingsRepo.Get(ctx, settingRequireAuth); err == nil && val == "false" {
		if user.Role != model.RoleAdmin {
			log.Warn().Str("user", user.Username).Msg("non-admin login blocked during maintenance")
			http.Error(w, "Login is temporarily disabled for maintenance", http.StatusServiceUnavailable)
			return
		}
	}

	// Generate JWT
	jwt, err := middleware.GenerateJWT(user.ID, user.Role, h.cfg.JWTSecret)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate JWT")
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    jwt,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400, // 24h
	})

	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "login",
		ResourceType: "user",
		ResourceID:   user.ID.String(),
		ResourceName: user.Username,
		IPAddress:    clientIP(r),
	})

	log.Info().Str("user", user.Username).Msg("user authenticated")

	// Redirect to dashboard with token
	dashboardURL := fmt.Sprintf("%s/auth/callback?token=%s", h.cfg.BaseURL, jwt)
	http.Redirect(w, r, dashboardURL, http.StatusTemporaryRedirect)
}

// Me returns the current authenticated user's info.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	now := time.Now()
	user.LastLoginAt = &now

	writeJSON(w, http.StatusOK, user.ToResponse())
}
