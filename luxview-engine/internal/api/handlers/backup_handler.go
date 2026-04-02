package handlers

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

type BackupHandler struct {
	backupSvc *service.BackupService
	auditSvc  *service.AuditService
}

func NewBackupHandler(backupSvc *service.BackupService, auditSvc *service.AuditService) *BackupHandler {
	return &BackupHandler{backupSvc: backupSvc, auditSvc: auditSvc}
}

// List returns paginated backup history.
func (h *BackupHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	backups, total, err := h.backupSvc.List(ctx, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list backups")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"backups": backups,
		"total":   total,
	})
}

// Get returns a single backup by ID.
func (h *BackupHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid backup ID")
		return
	}

	backup, err := h.backupSvc.FindByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get backup")
		return
	}
	if backup == nil {
		writeError(w, http.StatusNotFound, "backup not found")
		return
	}

	writeJSON(w, http.StatusOK, backup)
}

// Trigger starts a manual backup.
func (h *BackupHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	log := logger.With("backup")
	ctx := r.Context()
	user := middleware.GetUser(ctx)

	var req struct {
		Databases []string `json:"databases"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// If no databases specified, use configured ones
	if len(req.Databases) == 0 {
		settings := h.backupSvc.GetSettings(ctx)
		req.Databases = settings.Databases
	}

	if len(req.Databases) == 0 {
		writeError(w, http.StatusBadRequest, "no databases specified")
		return
	}

	// Validate databases
	for _, db := range req.Databases {
		if !model.IsValidDatabase(db) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid database: %s", db))
			return
		}
	}

	if h.backupSvc.IsRunning() {
		writeError(w, http.StatusConflict, "a backup or restore operation is already running")
		return
	}

	userID := user.ID
	go func() {
		backup, err := h.backupSvc.Run(ctx, req.Databases, model.BackupTriggerManual, &userID)
		if err != nil {
			log.Error().Err(err).Msg("manual backup failed")
			return
		}

		h.auditSvc.Log(ctx, service.AuditEntry{
			ActorID:       user.ID,
			ActorUsername: user.Username,
			Action:        "create",
			ResourceType:  "backup",
			ResourceID:    backup.ID.String(),
			ResourceName:  "manual backup",
			NewValues:     map[string]interface{}{"databases": req.Databases},
			IPAddress:     clientIP(r),
		})
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"message": "backup started"})
}

// Delete removes a backup.
func (h *BackupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := middleware.GetUser(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid backup ID")
		return
	}

	if err := h.backupSvc.Delete(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete backup")
		return
	}

	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:       user.ID,
		ActorUsername: user.Username,
		Action:        "delete",
		ResourceType:  "backup",
		ResourceID:    id.String(),
		ResourceName:  "backup",
		IPAddress:     clientIP(r),
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "backup deleted"})
}

// Restore restores from a backup with confirmation.
func (h *BackupHandler) Restore(w http.ResponseWriter, r *http.Request) {
	log := logger.With("backup")
	ctx := r.Context()
	user := middleware.GetUser(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid backup ID")
		return
	}

	var req struct {
		Confirm string `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Confirm != "RESTORE" {
		writeError(w, http.StatusBadRequest, "confirmation text must be 'RESTORE'")
		return
	}

	if h.backupSvc.IsRunning() {
		writeError(w, http.StatusConflict, "a backup or restore operation is already running")
		return
	}

	go func() {
		err := h.backupSvc.Restore(ctx, id)
		if err != nil {
			log.Error().Err(err).Str("backup_id", id.String()).Msg("restore failed")
			return
		}

		h.auditSvc.Log(ctx, service.AuditEntry{
			ActorID:       user.ID,
			ActorUsername: user.Username,
			Action:        "restore",
			ResourceType:  "backup",
			ResourceID:    id.String(),
			ResourceName:  "backup restore",
			IPAddress:     clientIP(r),
		})
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"message": "restore started"})
}

// Download streams the backup directory as a tar.gz archive.
func (h *BackupHandler) Download(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid backup ID")
		return
	}

	backup, err := h.backupSvc.FindByID(ctx, id)
	if err != nil || backup == nil {
		writeError(w, http.StatusNotFound, "backup not found")
		return
	}

	if backup.Status != model.BackupStatusCompleted {
		writeError(w, http.StatusBadRequest, "backup is not completed")
		return
	}

	if _, err := os.Stat(backup.FilePath); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "backup files not found on disk")
		return
	}

	dirName := filepath.Base(backup.FilePath)
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.tar.gz", dirName))

	gw := gzip.NewWriter(w)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	filepath.Walk(backup.FilePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(backup.FilePath, path)
		header := &tar.Header{
			Name: relPath,
			Size: info.Size(),
			Mode: int64(info.Mode()),
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}

// GetSettings returns backup configuration.
func (h *BackupHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings := h.backupSvc.GetSettings(r.Context())
	writeJSON(w, http.StatusOK, settings)
}

// UpdateSettings updates backup configuration.
func (h *BackupHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := middleware.GetUser(ctx)

	var req model.BackupSettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Schedule != "" && !model.IsValidSchedule(req.Schedule) {
		writeError(w, http.StatusBadRequest, "invalid schedule: must be daily, weekly, or monthly")
		return
	}
	if req.RetentionDays != 0 && !model.IsValidRetention(req.RetentionDays) {
		writeError(w, http.StatusBadRequest, "invalid retention: must be 7, 14, 30, or 60")
		return
	}
	for _, db := range req.Databases {
		if !model.IsValidDatabase(db) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid database: %s", db))
			return
		}
	}

	if req.Schedule == "" {
		req.Schedule = "daily"
	}
	if req.RetentionDays == 0 {
		req.RetentionDays = 30
	}

	if err := h.backupSvc.SaveSettings(ctx, req); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update settings")
		return
	}

	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:       user.ID,
		ActorUsername: user.Username,
		Action:        "update",
		ResourceType:  "setting",
		ResourceID:    "backup",
		ResourceName:  "backup settings",
		NewValues:     map[string]interface{}{"enabled": req.Enabled, "schedule": req.Schedule, "retention_days": req.RetentionDays, "databases": req.Databases},
		IPAddress:     clientIP(r),
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "backup settings updated"})
}

// formatBytes converts bytes to human-readable string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
