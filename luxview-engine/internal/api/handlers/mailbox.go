package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

var validLocalPart = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?$`)

type MailboxHandler struct {
	mailboxRepo *repository.MailboxRepo
	serviceRepo *repository.ServiceRepo
	appRepo     *repository.AppRepo
	provisioner *service.Provisioner
	auditSvc    *service.AuditService
	domain      string
}

func NewMailboxHandler(
	mailboxRepo *repository.MailboxRepo,
	serviceRepo *repository.ServiceRepo,
	appRepo *repository.AppRepo,
	provisioner *service.Provisioner,
	auditSvc *service.AuditService,
	domain string,
) *MailboxHandler {
	return &MailboxHandler{
		mailboxRepo: mailboxRepo,
		serviceRepo: serviceRepo,
		appRepo:     appRepo,
		provisioner: provisioner,
		auditSvc:    auditSvc,
		domain:      domain,
	}
}

// List returns all mailboxes for an email service.
func (h *MailboxHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	svcID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid service ID")
		return
	}

	svc, err := h.serviceRepo.FindByID(ctx, svcID)
	if err != nil || svc == nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}
	if svc.ServiceType != model.ServiceEmail {
		writeError(w, http.StatusBadRequest, "service is not an email service")
		return
	}

	app, err := h.appRepo.FindByID(ctx, svc.AppID)
	if err != nil || app == nil || app.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	mailboxes, err := h.mailboxRepo.ListByServiceID(ctx, svcID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list mailboxes")
		return
	}

	if mailboxes == nil {
		mailboxes = []model.Mailbox{}
	}

	writeJSON(w, http.StatusOK, mailboxes)
}

// Create provisions a new mailbox in docker-mailserver.
func (h *MailboxHandler) Create(w http.ResponseWriter, r *http.Request) {
	log := logger.With("mailbox")
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	svcID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid service ID")
		return
	}

	svc, err := h.serviceRepo.FindByID(ctx, svcID)
	if err != nil || svc == nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}
	if svc.ServiceType != model.ServiceEmail {
		writeError(w, http.StatusBadRequest, "service is not an email service")
		return
	}

	app, err := h.appRepo.FindByID(ctx, svc.AppID)
	if err != nil || app == nil || app.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	// Plan enforcement
	user := middleware.GetUser(ctx)
	if user.Plan != nil && user.Plan.MaxMailboxesPerApp > 0 {
		count, _ := h.mailboxRepo.CountByServiceID(ctx, svcID)
		if count >= user.Plan.MaxMailboxesPerApp {
			writeError(w, http.StatusForbidden, fmt.Sprintf("mailbox limit reached: your %s plan allows max %d mailboxes per app", user.Plan.Name, user.Plan.MaxMailboxesPerApp))
			return
		}
	} else if user.Plan != nil && user.Plan.MaxMailboxesPerApp == 0 {
		writeError(w, http.StatusForbidden, "your plan does not include email mailboxes")
		return
	}

	var req model.CreateMailboxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	localPart := strings.TrimSpace(strings.ToLower(req.LocalPart))
	if localPart == "" {
		writeError(w, http.StatusBadRequest, "local_part is required")
		return
	}
	if len(localPart) > 64 {
		writeError(w, http.StatusBadRequest, "local_part must be 64 characters or less")
		return
	}
	if !validLocalPart.MatchString(localPart) {
		writeError(w, http.StatusBadRequest, "local_part contains invalid characters (use letters, numbers, dots, hyphens, underscores)")
		return
	}

	address := fmt.Sprintf("%s@%s.%s", localPart, app.Subdomain, h.domain)

	// Generate password
	password := generateMailboxPassword()

	// Create in docker-mailserver
	if err := h.provisioner.MailserverAddEmail(ctx, address, password); err != nil {
		log.Error().Err(err).Str("address", address).Msg("failed to add mailbox to mailserver")
		writeError(w, http.StatusInternalServerError, "failed to create mailbox")
		return
	}

	// Set quota if plan defines it
	if user.Plan != nil && user.Plan.MaxMailboxStorage != "" {
		if err := h.provisioner.MailserverSetQuota(ctx, address, user.Plan.MaxMailboxStorage); err != nil {
			log.Warn().Err(err).Str("address", address).Msg("failed to set mailbox quota")
		}
	}

	// Save to DB
	mailbox := &model.Mailbox{
		ServiceID: svcID,
		Address:   address,
	}
	if err := h.mailboxRepo.Create(ctx, mailbox); err != nil {
		// Rollback: remove from mailserver
		_ = h.provisioner.MailserverUpdatePassword(ctx, address, "")
		log.Error().Err(err).Msg("failed to save mailbox to database")
		writeError(w, http.StatusInternalServerError, "failed to create mailbox")
		return
	}

	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "create",
		ResourceType: "mailbox",
		ResourceID:   mailbox.ID.String(),
		ResourceName: address,
		NewValues:    map[string]string{"address": address, "app": app.Subdomain},
		IPAddress:    clientIP(r),
	})

	log.Info().Str("address", address).Msg("mailbox created")

	// Return mailbox + password (shown once)
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":         mailbox.ID,
		"service_id": mailbox.ServiceID,
		"address":    mailbox.Address,
		"password":   password,
		"created_at": mailbox.CreatedAt,
	})
}

// Delete removes a mailbox from docker-mailserver and the DB.
func (h *MailboxHandler) Delete(w http.ResponseWriter, r *http.Request) {
	log := logger.With("mailbox")
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	mbID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid mailbox ID")
		return
	}

	mailbox, err := h.mailboxRepo.FindByID(ctx, mbID)
	if err != nil || mailbox == nil {
		writeError(w, http.StatusNotFound, "mailbox not found")
		return
	}

	svc, err := h.serviceRepo.FindByID(ctx, mailbox.ServiceID)
	if err != nil || svc == nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	app, err := h.appRepo.FindByID(ctx, svc.AppID)
	if err != nil || app == nil || app.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	// Remove from docker-mailserver
	if err := h.provisioner.MailserverDeleteEmail(ctx, mailbox.Address); err != nil {
		log.Warn().Err(err).Str("address", mailbox.Address).Msg("failed to delete mailbox from mailserver")
	}

	// Remove from DB
	if err := h.mailboxRepo.Delete(ctx, mbID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete mailbox")
		return
	}

	user := middleware.GetUser(ctx)
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "delete",
		ResourceType: "mailbox",
		ResourceID:   mailbox.ID.String(),
		ResourceName: mailbox.Address,
		OldValues:    map[string]string{"address": mailbox.Address},
		IPAddress:    clientIP(r),
	})

	log.Info().Str("address", mailbox.Address).Msg("mailbox deleted")
	writeJSON(w, http.StatusOK, map[string]string{"message": "mailbox deleted"})
}

// ResetPassword generates a new password for a mailbox.
func (h *MailboxHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	log := logger.With("mailbox")
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	mbID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid mailbox ID")
		return
	}

	mailbox, err := h.mailboxRepo.FindByID(ctx, mbID)
	if err != nil || mailbox == nil {
		writeError(w, http.StatusNotFound, "mailbox not found")
		return
	}

	svc, err := h.serviceRepo.FindByID(ctx, mailbox.ServiceID)
	if err != nil || svc == nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	app, err := h.appRepo.FindByID(ctx, svc.AppID)
	if err != nil || app == nil || app.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	newPassword := generateMailboxPassword()

	if err := h.provisioner.MailserverUpdatePassword(ctx, mailbox.Address, newPassword); err != nil {
		log.Error().Err(err).Str("address", mailbox.Address).Msg("failed to update mailbox password")
		writeError(w, http.StatusInternalServerError, "failed to reset password")
		return
	}

	user := middleware.GetUser(ctx)
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "update",
		ResourceType: "mailbox",
		ResourceID:   mailbox.ID.String(),
		ResourceName: mailbox.Address,
		NewValues:    map[string]string{"action": "password_reset"},
		IPAddress:    clientIP(r),
	})

	log.Info().Str("address", mailbox.Address).Msg("mailbox password reset")
	writeJSON(w, http.StatusOK, map[string]string{
		"address":  mailbox.Address,
		"password": newPassword,
	})
}

func generateMailboxPassword() string {
	// 16 hex chars = 8 bytes of entropy
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
