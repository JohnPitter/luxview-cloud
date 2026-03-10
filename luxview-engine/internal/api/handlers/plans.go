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
	"github.com/luxview/engine/pkg/logger"
)

type PlanHandler struct {
	planRepo *repository.PlanRepo
	userRepo *repository.UserRepo
	appRepo  *repository.AppRepo
	auditSvc *service.AuditService
}

func NewPlanHandler(planRepo *repository.PlanRepo, userRepo *repository.UserRepo, appRepo *repository.AppRepo, auditSvc *service.AuditService) *PlanHandler {
	return &PlanHandler{
		planRepo: planRepo,
		userRepo: userRepo,
		appRepo:  appRepo,
		auditSvc: auditSvc,
	}
}

// ListActive returns all active plans (public endpoint for landing page).
func (h *PlanHandler) ListActive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	plans, err := h.planRepo.ListActive(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list plans")
		return
	}

	writeJSON(w, http.StatusOK, plans)
}

// ListAll returns all plans including inactive (admin only).
func (h *PlanHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	plans, err := h.planRepo.ListAll(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list plans")
		return
	}

	writeJSON(w, http.StatusOK, plans)
}

// Create creates a new plan (admin only).
func (h *PlanHandler) Create(w http.ResponseWriter, r *http.Request) {
	log := logger.With("plans")
	ctx := r.Context()

	var req model.CreatePlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.MaxApps < 1 {
		writeError(w, http.StatusBadRequest, "max_apps must be at least 1")
		return
	}

	if req.Features == nil {
		req.Features = []string{}
	}

	plan := &model.Plan{
		Name:                req.Name,
		Description:         req.Description,
		Price:               req.Price,
		Currency:            req.Currency,
		BillingCycle:        req.BillingCycle,
		MaxApps:             req.MaxApps,
		MaxCPUPerApp:        req.MaxCPUPerApp,
		MaxMemoryPerApp:     req.MaxMemoryPerApp,
		MaxDiskPerApp:       req.MaxDiskPerApp,
		MaxServicesPerApp:   req.MaxServicesPerApp,
		AutoDeployEnabled:   req.AutoDeployEnabled,
		CustomDomainEnabled: req.CustomDomainEnabled,
		PriorityBuilds:      req.PriorityBuilds,
		Highlighted:         req.Highlighted,
		SortOrder:           req.SortOrder,
		Features:            req.Features,
		IsActive:            true,
		IsDefault:           false,
	}

	if err := h.planRepo.Create(ctx, plan); err != nil {
		log.Error().Err(err).Msg("failed to create plan")
		writeError(w, http.StatusInternalServerError, "failed to create plan")
		return
	}

	user := middleware.GetUser(ctx)
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "create",
		ResourceType: "plan",
		ResourceID:   plan.ID.String(),
		ResourceName: plan.Name,
		NewValues:    map[string]interface{}{"name": plan.Name, "price": plan.Price},
		IPAddress:    clientIP(r),
	})

	log.Info().Str("plan", plan.Name).Msg("plan created")
	writeJSON(w, http.StatusCreated, plan)
}

// Update updates an existing plan (admin only).
func (h *PlanHandler) Update(w http.ResponseWriter, r *http.Request) {
	log := logger.With("plans")
	ctx := r.Context()

	planID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid plan ID")
		return
	}

	plan, err := h.planRepo.FindByID(ctx, planID)
	if err != nil || plan == nil {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}

	// Capture old values for audit
	oldPlanValues := map[string]interface{}{"name": plan.Name, "price": plan.Price, "isActive": plan.IsActive, "isDefault": plan.IsDefault}

	var req model.UpdatePlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != nil {
		plan.Name = *req.Name
	}
	if req.Description != nil {
		plan.Description = *req.Description
	}
	if req.Price != nil {
		plan.Price = *req.Price
	}
	if req.Currency != nil {
		plan.Currency = *req.Currency
	}
	if req.BillingCycle != nil {
		plan.BillingCycle = *req.BillingCycle
	}
	if req.MaxApps != nil {
		plan.MaxApps = *req.MaxApps
	}
	if req.MaxCPUPerApp != nil {
		plan.MaxCPUPerApp = *req.MaxCPUPerApp
	}
	if req.MaxMemoryPerApp != nil {
		plan.MaxMemoryPerApp = *req.MaxMemoryPerApp
	}
	if req.MaxDiskPerApp != nil {
		plan.MaxDiskPerApp = *req.MaxDiskPerApp
	}
	if req.MaxServicesPerApp != nil {
		plan.MaxServicesPerApp = *req.MaxServicesPerApp
	}
	if req.AutoDeployEnabled != nil {
		plan.AutoDeployEnabled = *req.AutoDeployEnabled
	}
	if req.CustomDomainEnabled != nil {
		plan.CustomDomainEnabled = *req.CustomDomainEnabled
	}
	if req.PriorityBuilds != nil {
		plan.PriorityBuilds = *req.PriorityBuilds
	}
	if req.Highlighted != nil {
		plan.Highlighted = *req.Highlighted
	}
	if req.SortOrder != nil {
		plan.SortOrder = *req.SortOrder
	}
	if req.Features != nil {
		plan.Features = req.Features
	}
	if req.IsActive != nil {
		plan.IsActive = *req.IsActive
	}
	if req.IsDefault != nil {
		plan.IsDefault = *req.IsDefault
	}

	if err := h.planRepo.Update(ctx, plan); err != nil {
		log.Error().Err(err).Msg("failed to update plan")
		writeError(w, http.StatusInternalServerError, "failed to update plan")
		return
	}

	user := middleware.GetUser(ctx)
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "update",
		ResourceType: "plan",
		ResourceID:   plan.ID.String(),
		ResourceName: plan.Name,
		OldValues:    oldPlanValues,
		NewValues:    map[string]interface{}{"name": plan.Name, "price": plan.Price, "isActive": plan.IsActive, "isDefault": plan.IsDefault},
		IPAddress:    clientIP(r),
	})

	log.Info().Str("plan", plan.Name).Msg("plan updated")
	writeJSON(w, http.StatusOK, plan)
}

// Delete soft-deletes a plan (admin only).
func (h *PlanHandler) Delete(w http.ResponseWriter, r *http.Request) {
	log := logger.With("plans")
	ctx := r.Context()

	planID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid plan ID")
		return
	}

	plan, err := h.planRepo.FindByID(ctx, planID)
	if err != nil || plan == nil {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}

	if err := h.planRepo.Delete(ctx, planID); err != nil {
		log.Error().Err(err).Msg("failed to delete plan")
		writeError(w, http.StatusInternalServerError, "failed to delete plan")
		return
	}

	user := middleware.GetUser(ctx)
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "delete",
		ResourceType: "plan",
		ResourceID:   plan.ID.String(),
		ResourceName: plan.Name,
		OldValues:    map[string]string{"name": plan.Name},
		IPAddress:    clientIP(r),
	})

	log.Info().Str("plan", plan.Name).Msg("plan soft-deleted")
	writeJSON(w, http.StatusOK, map[string]string{"message": "plan deleted"})
}

// SetDefault sets a plan as the default (admin only).
func (h *PlanHandler) SetDefault(w http.ResponseWriter, r *http.Request) {
	log := logger.With("plans")
	ctx := r.Context()

	planID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid plan ID")
		return
	}

	plan, err := h.planRepo.FindByID(ctx, planID)
	if err != nil || plan == nil {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}

	if err := h.planRepo.SetDefault(ctx, planID); err != nil {
		log.Error().Err(err).Msg("failed to set default plan")
		writeError(w, http.StatusInternalServerError, "failed to set default plan")
		return
	}

	user := middleware.GetUser(ctx)
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "update",
		ResourceType: "plan",
		ResourceID:   plan.ID.String(),
		ResourceName: plan.Name,
		NewValues:    map[string]interface{}{"isDefault": true},
		IPAddress:    clientIP(r),
	})

	log.Info().Str("plan", plan.Name).Msg("plan set as default")
	writeJSON(w, http.StatusOK, map[string]string{"message": "plan set as default"})
}

// AssignUserPlan assigns a plan to a user (admin only).
func (h *PlanHandler) AssignUserPlan(w http.ResponseWriter, r *http.Request) {
	log := logger.With("plans")
	ctx := r.Context()

	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var body struct {
		PlanID uuid.UUID `json:"plan_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate user exists
	user, err := h.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	// Validate plan exists
	plan, err := h.planRepo.FindByID(ctx, body.PlanID)
	if err != nil || plan == nil {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}

	if err := h.userRepo.UpdatePlanID(ctx, userID, body.PlanID); err != nil {
		log.Error().Err(err).Msg("failed to assign plan to user")
		writeError(w, http.StatusInternalServerError, "failed to assign plan")
		return
	}

	actor := middleware.GetUser(ctx)
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      actor.ID,
		ActorUsername: actor.Username,
		Action:       "update",
		ResourceType: "user",
		ResourceID:   user.ID.String(),
		ResourceName: user.Username,
		NewValues:    map[string]string{"planId": body.PlanID.String()},
		IPAddress:    clientIP(r),
	})

	log.Info().Str("user", user.Username).Str("plan", plan.Name).Msg("plan assigned to user")
	writeJSON(w, http.StatusOK, map[string]string{"message": "plan assigned"})
}
