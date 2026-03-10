package handlers

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"github.com/luxview/engine/internal/repository"
	dockerclient "github.com/luxview/engine/pkg/docker"
	"github.com/luxview/engine/pkg/logger"
)

type CleanupHandler struct {
	settingsRepo *repository.SettingsRepo
	docker       *dockerclient.Client
}

func NewCleanupHandler(settingsRepo *repository.SettingsRepo, docker *dockerclient.Client) *CleanupHandler {
	return &CleanupHandler{settingsRepo: settingsRepo, docker: docker}
}

type cleanupSettingsResponse struct {
	Enabled          bool `json:"enabled"`
	IntervalHours    int  `json:"interval_hours"`
	ThresholdPercent int  `json:"threshold_percent"`
}

type updateCleanupSettingsRequest struct {
	Enabled          *bool `json:"enabled"`
	IntervalHours    *int  `json:"interval_hours"`
	ThresholdPercent *int  `json:"threshold_percent"`
}

// GetCleanupSettings returns the current Docker cleanup configuration.
func (h *CleanupHandler) GetCleanupSettings(w http.ResponseWriter, r *http.Request) {
	log := logger.With("cleanup")
	ctx := r.Context()

	settings, err := h.settingsRepo.GetAll(ctx, "cleanup_")
	if err != nil {
		log.Error().Err(err).Msg("failed to get cleanup settings")
		writeError(w, http.StatusInternalServerError, "failed to get cleanup settings")
		return
	}

	intervalHours := 24
	if v, err := strconv.Atoi(settings["interval_hours"]); err == nil && v > 0 {
		intervalHours = v
	}

	thresholdPercent := 80
	if v, err := strconv.Atoi(settings["threshold_percent"]); err == nil && v > 0 {
		thresholdPercent = v
	}

	resp := cleanupSettingsResponse{
		Enabled:          settings["enabled"] == "true",
		IntervalHours:    intervalHours,
		ThresholdPercent: thresholdPercent,
	}

	writeJSON(w, http.StatusOK, resp)
}

// UpdateCleanupSettings updates Docker cleanup configuration.
func (h *CleanupHandler) UpdateCleanupSettings(w http.ResponseWriter, r *http.Request) {
	log := logger.With("cleanup")
	ctx := r.Context()

	var req updateCleanupSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Enabled != nil {
		val := "false"
		if *req.Enabled {
			val = "true"
		}
		if err := h.settingsRepo.Set(ctx, "cleanup_enabled", val, false); err != nil {
			log.Error().Err(err).Msg("failed to set cleanup enabled")
			writeError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}
	}

	if req.IntervalHours != nil {
		if *req.IntervalHours < 1 {
			writeError(w, http.StatusBadRequest, "interval must be at least 1 hour")
			return
		}
		if err := h.settingsRepo.Set(ctx, "cleanup_interval_hours", strconv.Itoa(*req.IntervalHours), false); err != nil {
			log.Error().Err(err).Msg("failed to set cleanup interval")
			writeError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}
	}

	if req.ThresholdPercent != nil {
		if *req.ThresholdPercent < 10 || *req.ThresholdPercent > 99 {
			writeError(w, http.StatusBadRequest, "threshold must be between 10 and 99")
			return
		}
		if err := h.settingsRepo.Set(ctx, "cleanup_threshold_percent", strconv.Itoa(*req.ThresholdPercent), false); err != nil {
			log.Error().Err(err).Msg("failed to set cleanup threshold")
			writeError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}
	}

	log.Info().Msg("cleanup settings updated")
	writeJSON(w, http.StatusOK, map[string]string{"message": "cleanup settings updated"})
}

// TriggerCleanup manually runs a Docker system prune.
func (h *CleanupHandler) TriggerCleanup(w http.ResponseWriter, r *http.Request) {
	log := logger.With("cleanup")
	ctx := r.Context()

	log.Info().Msg("manual docker cleanup triggered")

	result, err := h.docker.SystemPrune(ctx)
	if err != nil {
		log.Error().Err(err).Msg("manual docker prune failed")
		writeError(w, http.StatusInternalServerError, "docker prune failed")
		return
	}

	log.Info().
		Int("images_removed", result.ImagesRemoved).
		Int("containers_removed", result.ContainersRemoved).
		Int64("total_reclaimed", result.TotalReclaimed).
		Msg("manual docker prune completed")

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"images_removed":       result.ImagesRemoved,
		"containers_removed":   result.ContainersRemoved,
		"build_cache_reclaimed": result.BuildCacheReclaimed,
		"images_reclaimed":     result.ImagesReclaimed,
		"total_reclaimed":      result.TotalReclaimed,
	})
}

// DiskUsage returns current Docker disk usage breakdown.
func (h *CleanupHandler) DiskUsage(w http.ResponseWriter, _ *http.Request) {
	// Get overall disk info
	diskInfo := readDockerDiskUsage()

	writeJSON(w, http.StatusOK, diskInfo)
}

// readDockerDiskUsage gets Docker-specific disk usage via docker system df.
func readDockerDiskUsage() map[string]interface{} {
	result := map[string]interface{}{}

	// Root disk usage
	out, err := exec.Command("df", "-B1", "/").Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) >= 2 {
			fields := strings.Fields(lines[1])
			if len(fields) >= 6 {
				total, _ := strconv.ParseInt(fields[1], 10, 64)
				used, _ := strconv.ParseInt(fields[2], 10, 64)
				available, _ := strconv.ParseInt(fields[3], 10, 64)
				result["disk_total"] = total
				result["disk_used"] = used
				result["disk_available"] = available
				result["disk_percent"] = strings.TrimSuffix(fields[4], "%")
			}
		}
	}

	// Docker system df (parseable)
	out, err = exec.Command("docker", "system", "df", "--format", "{{.Type}}\t{{.Size}}\t{{.Reclaimable}}").Output()
	if err == nil {
		dockerBreakdown := []map[string]string{}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			parts := strings.SplitN(line, "\t", 3)
			if len(parts) >= 3 {
				dockerBreakdown = append(dockerBreakdown, map[string]string{
					"type":        parts[0],
					"size":        parts[1],
					"reclaimable": parts[2],
				})
			}
		}
		result["docker"] = dockerBreakdown
	}

	// Image count
	imgOut, err := exec.Command("docker", "images", "--format", "{{.ID}}").Output()
	if err == nil {
		imgs := strings.Split(strings.TrimSpace(string(imgOut)), "\n")
		count := 0
		for _, img := range imgs {
			if strings.TrimSpace(img) != "" {
				count++
			}
		}
		result["image_count"] = count
	}

	// Active container count
	cntOut, err := exec.Command("docker", "ps", "-q").Output()
	if err == nil {
		cnts := strings.Split(strings.TrimSpace(string(cntOut)), "\n")
		count := 0
		for _, c := range cnts {
			if strings.TrimSpace(c) != "" {
				count++
			}
		}
		result["active_container_count"] = count
	}

	return result
}
