package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/crypto"
	"github.com/luxview/engine/pkg/logger"
)

type ServiceHandler struct {
	serviceRepo   *repository.ServiceRepo
	appRepo       *repository.AppRepo
	provisioner   *service.Provisioner
	encryptionKey []byte
	auditSvc      *service.AuditService
}

func NewServiceHandler(
	serviceRepo *repository.ServiceRepo,
	appRepo *repository.AppRepo,
	provisioner *service.Provisioner,
	encryptionKey []byte,
	auditSvc *service.AuditService,
) *ServiceHandler {
	return &ServiceHandler{
		serviceRepo:   serviceRepo,
		appRepo:       appRepo,
		provisioner:   provisioner,
		encryptionKey: encryptionKey,
		auditSvc:      auditSvc,
	}
}

// ListAll lists all services for the authenticated user across all apps.
func (h *ServiceHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	services, err := h.serviceRepo.ListByUserID(ctx, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list services")
		return
	}

	// Decrypt credentials for response
	for i := range services {
		var encrypted string
		if err := json.Unmarshal(services[i].Credentials, &encrypted); err == nil {
			if decrypted, err := crypto.Decrypt(encrypted, h.encryptionKey); err == nil {
				_ = json.Unmarshal([]byte(decrypted), &services[i].CredentialsPlain)
			}
		}
	}

	// Attach app name to each service
	type serviceWithApp struct {
		model.AppService
		AppName      string `json:"app_name"`
		AppSubdomain string `json:"app_subdomain"`
	}

	result := make([]serviceWithApp, 0, len(services))
	for _, svc := range services {
		s := serviceWithApp{AppService: svc}
		if app, err := h.appRepo.FindByID(ctx, svc.AppID); err == nil && app != nil {
			s.AppName = app.Name
			s.AppSubdomain = app.Subdomain
		}
		result = append(result, s)
	}

	writeJSON(w, http.StatusOK, result)
}

// Create provisions a new service for an app.
func (h *ServiceHandler) Create(w http.ResponseWriter, r *http.Request) {
	log := logger.With("services")
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid app ID")
		return
	}

	app, err := h.appRepo.FindByID(ctx, appID)
	if err != nil || app == nil {
		writeError(w, http.StatusNotFound, "app not found")
		return
	}
	if app.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	// Plan enforcement: check max services per app
	user := middleware.GetUser(ctx)
	if user.Plan != nil {
		existingServices, _ := h.serviceRepo.ListByAppID(ctx, appID)
		if len(existingServices) >= user.Plan.MaxServicesPerApp {
			writeError(w, http.StatusForbidden, fmt.Sprintf("Plan limit reached: your %s plan allows max %d services per app", user.Plan.Name, user.Plan.MaxServicesPerApp))
			return
		}
	}

	var req model.CreateServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate service type
	switch req.ServiceType {
	case model.ServicePostgres, model.ServiceRedis, model.ServiceMongoDB, model.ServiceRabbitMQ, model.ServiceStorage:
		// valid
	default:
		writeError(w, http.StatusBadRequest, "invalid service type")
		return
	}

	svc, err := h.provisioner.Provision(ctx, appID, req.ServiceType)
	if err != nil {
		log.Error().Err(err).Msg("failed to provision service")
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "create",
		ResourceType: "service",
		ResourceID:   svc.ID.String(),
		ResourceName: string(req.ServiceType),
		NewValues:    map[string]string{"type": string(req.ServiceType), "appSubdomain": app.Subdomain},
		IPAddress:    clientIP(r),
	})

	log.Info().Str("app", app.Subdomain).Str("type", string(req.ServiceType)).Msg("service provisioned")
	writeJSON(w, http.StatusCreated, svc)
}

// List lists all services for an app.
func (h *ServiceHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid app ID")
		return
	}

	app, err := h.appRepo.FindByID(ctx, appID)
	if err != nil || app == nil {
		writeError(w, http.StatusNotFound, "app not found")
		return
	}
	if app.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	services, err := h.serviceRepo.ListByAppID(ctx, appID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list services")
		return
	}

	// Decrypt credentials for response
	for i := range services {
		var encrypted string
		if err := json.Unmarshal(services[i].Credentials, &encrypted); err == nil {
			if decrypted, err := crypto.Decrypt(encrypted, h.encryptionKey); err == nil {
				_ = json.Unmarshal([]byte(decrypted), &services[i].CredentialsPlain)
			}
		}
	}

	writeJSON(w, http.StatusOK, services)
}

// Delete removes a service.
func (h *ServiceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	log := logger.With("services")
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

	app, err := h.appRepo.FindByID(ctx, svc.AppID)
	if err != nil || app == nil || app.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	if err := h.provisioner.Deprovision(ctx, svc); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove service")
		return
	}

	user := middleware.GetUser(ctx)
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "delete",
		ResourceType: "service",
		ResourceID:   svc.ID.String(),
		ResourceName: string(svc.ServiceType),
		OldValues:    map[string]string{"type": string(svc.ServiceType)},
		IPAddress:    clientIP(r),
	})

	log.Info().Str("service", svcID.String()).Msg("service removed")
	writeJSON(w, http.StatusOK, map[string]string{"message": "service removed"})
}
