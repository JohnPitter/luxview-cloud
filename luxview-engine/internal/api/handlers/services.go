package handlers

import (
	"encoding/json"
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
}

func NewServiceHandler(
	serviceRepo *repository.ServiceRepo,
	appRepo *repository.AppRepo,
	provisioner *service.Provisioner,
	encryptionKey []byte,
) *ServiceHandler {
	return &ServiceHandler{
		serviceRepo:   serviceRepo,
		appRepo:       appRepo,
		provisioner:   provisioner,
		encryptionKey: encryptionKey,
	}
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

	var req model.CreateServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate service type
	switch req.ServiceType {
	case model.ServicePostgres, model.ServiceRedis, model.ServiceMongoDB, model.ServiceRabbitMQ:
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

	log.Info().Str("service", svcID.String()).Msg("service removed")
	writeJSON(w, http.StatusOK, map[string]string{"message": "service removed"})
}
