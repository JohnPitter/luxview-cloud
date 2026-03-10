package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

type AdminHandler struct {
	userRepo    *repository.UserRepo
	appRepo     *repository.AppRepo
	deployRepo  *repository.DeploymentRepo
	serviceRepo *repository.ServiceRepo
	container   *service.ContainerManager
	provisioner *service.Provisioner
	auditSvc    *service.AuditService
}

func NewAdminHandler(
	userRepo *repository.UserRepo,
	appRepo *repository.AppRepo,
	deployRepo *repository.DeploymentRepo,
	serviceRepo *repository.ServiceRepo,
	container *service.ContainerManager,
	provisioner *service.Provisioner,
	auditSvc *service.AuditService,
) *AdminHandler {
	return &AdminHandler{
		userRepo:    userRepo,
		appRepo:     appRepo,
		deployRepo:  deployRepo,
		serviceRepo: serviceRepo,
		container:   container,
		provisioner: provisioner,
		auditSvc:    auditSvc,
	}
}

// ListUsers lists all users (admin only).
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	users, total, err := h.userRepo.ListAll(ctx, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	// Convert to response format (no sensitive fields)
	var responses []model.UserResponse
	for _, u := range users {
		responses = append(responses, u.ToResponse())
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"users": responses,
		"total": total,
	})
}

// Stats returns global platform stats.
func (h *AdminHandler) Stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, userTotal, _ := h.userRepo.ListAll(ctx, 1, 0)
	appTotal, _ := h.appRepo.CountAll(ctx)
	runningApps, _ := h.appRepo.CountByStatus(ctx, model.AppStatusRunning)
	deployTotal, _ := h.deployRepo.CountAll(ctx)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total_users":       userTotal,
		"total_apps":        appTotal,
		"running_apps":      runningApps,
		"total_deployments": deployTotal,
	})
}

// ForceDeleteApp force-deletes any app (admin only).
func (h *AdminHandler) ForceDeleteApp(w http.ResponseWriter, r *http.Request) {
	log := logger.With("admin")
	ctx := r.Context()

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

	if app.ContainerID != "" {
		_ = h.container.Stop(ctx, app.ContainerID)
		_ = h.container.Remove(ctx, app.ContainerID)
	}

	// Deprovision all associated services (databases, buckets, etc.)
	services, err := h.serviceRepo.ListByAppID(ctx, appID)
	if err == nil {
		for i := range services {
			if depErr := h.provisioner.Deprovision(ctx, &services[i]); depErr != nil {
				log.Warn().Err(depErr).Str("service_id", services[i].ID.String()).Msg("failed to deprovision service during force delete")
			}
		}
	}

	if err := h.appRepo.Delete(ctx, appID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete app")
		return
	}

	user := middleware.GetUser(ctx)
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "delete",
		ResourceType: "app",
		ResourceID:   app.ID.String(),
		ResourceName: app.Subdomain,
		OldValues:    map[string]string{"name": app.Name, "subdomain": app.Subdomain, "owner": app.UserID.String()},
		IPAddress:    clientIP(r),
	})

	log.Info().Str("app", app.Subdomain).Msg("admin force-deleted app")
	writeJSON(w, http.StatusOK, map[string]string{"message": "app force deleted"})
}

// UpdateUserRole changes a user's role (admin only).
func (h *AdminHandler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var body struct {
		Role model.UserRole `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Role != model.RoleUser && body.Role != model.RoleAdmin {
		writeError(w, http.StatusBadRequest, "role must be 'user' or 'admin'")
		return
	}

	user, err := h.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	oldRole := user.Role

	if err := h.userRepo.UpdateRole(ctx, userID, body.Role); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update role")
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
		OldValues:    map[string]string{"role": string(oldRole)},
		NewValues:    map[string]string{"role": string(body.Role)},
		IPAddress:    clientIP(r),
	})

	log := logger.With("admin")
	log.Info().Str("user", user.Username).Str("role", string(body.Role)).Msg("user role updated")
	writeJSON(w, http.StatusOK, map[string]string{"message": "role updated"})
}

// Platform services reserve: ~1 CPU core, 2GB RAM for postgres, redis, mongo, rabbitmq, minio, traefik, engine, dashboard.
const (
	platformReservedCPU    = 1.0
	platformReservedMemory = 2 * 1024 * 1024 * 1024 // 2GB
)

// UpdateAppLimits changes an app's resource limits (admin only).
// Validates that the new limits don't exceed VPS capacity.
func (h *AdminHandler) UpdateAppLimits(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid app ID")
		return
	}

	var body struct {
		ResourceLimits model.ResourceLimits `json:"resource_limits"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	app, err := h.appRepo.FindByID(ctx, appID)
	if err != nil || app == nil {
		writeError(w, http.StatusNotFound, "app not found")
		return
	}

	// --- Validate against VPS capacity ---
	hostCPU, hostMem := readHostResources()
	availableCPU := float64(hostCPU) - platformReservedCPU
	availableMem := hostMem - platformReservedMemory

	// Sum resources allocated by OTHER apps (exclude current app)
	allApps, _, _ := h.appRepo.ListAll(ctx, 1000, 0)
	var otherCPU float64
	var otherMem int64
	for _, a := range allApps {
		if a.ID == appID {
			continue
		}
		if a.ResourceLimits.CPU != "" {
			if c, err := strconv.ParseFloat(a.ResourceLimits.CPU, 64); err == nil {
				otherCPU += c
			}
		} else {
			otherCPU += 0.5
		}
		if a.ResourceLimits.Memory != "" {
			otherMem += parseMemoryString(a.ResourceLimits.Memory)
		} else {
			otherMem += 512 * 1024 * 1024
		}
	}

	// Parse requested limits
	var reqCPU float64 = 0.5
	if body.ResourceLimits.CPU != "" {
		if c, err := strconv.ParseFloat(body.ResourceLimits.CPU, 64); err == nil {
			reqCPU = c
		}
	}
	var reqMem int64 = 512 * 1024 * 1024
	if body.ResourceLimits.Memory != "" {
		reqMem = parseMemoryString(body.ResourceLimits.Memory)
	}

	if otherCPU+reqCPU > availableCPU {
		writeError(w, http.StatusBadRequest, fmt.Sprintf(
			"CPU limit exceeds VPS capacity. Available for apps: %.1f cores, already allocated: %.1f, requested: %.1f",
			availableCPU, otherCPU, reqCPU))
		return
	}
	if otherMem+reqMem > availableMem {
		writeError(w, http.StatusBadRequest, fmt.Sprintf(
			"Memory limit exceeds VPS capacity. Available for apps: %s, already allocated: %s, requested: %s",
			formatBytesGo(availableMem), formatBytesGo(otherMem), formatBytesGo(reqMem)))
		return
	}

	oldLimits := map[string]string{"cpu": app.ResourceLimits.CPU, "memory": app.ResourceLimits.Memory, "disk": app.ResourceLimits.Disk}

	if err := h.appRepo.UpdateResourceLimits(ctx, appID, body.ResourceLimits); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update limits")
		return
	}

	actor := middleware.GetUser(ctx)
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      actor.ID,
		ActorUsername: actor.Username,
		Action:       "update",
		ResourceType: "app",
		ResourceID:   app.ID.String(),
		ResourceName: app.Subdomain,
		OldValues:    oldLimits,
		NewValues:    map[string]string{"cpu": body.ResourceLimits.CPU, "memory": body.ResourceLimits.Memory, "disk": body.ResourceLimits.Disk},
		IPAddress:    clientIP(r),
	})

	log := logger.With("admin")
	log.Info().Str("app", app.Subdomain).Msg("app resource limits updated")
	writeJSON(w, http.StatusOK, map[string]string{"message": "limits updated"})
}

func formatBytesGo(b int64) string {
	const gb = 1024 * 1024 * 1024
	const mb = 1024 * 1024
	if b >= gb {
		return fmt.Sprintf("%.1fGB", float64(b)/float64(gb))
	}
	return fmt.Sprintf("%dMB", b/mb)
}

// ListAllApps returns all apps across all users (admin only).
func (h *AdminHandler) ListAllApps(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	apps, total, err := h.appRepo.ListAll(ctx, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list apps")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"apps":  apps,
		"total": total,
	})
}

// VPSInfo returns host system information (CPU, RAM, disk) for the admin panel.
func (h *AdminHandler) VPSInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	hostCPU, hostMem := readHostResources()

	availableCPU := float64(hostCPU) - platformReservedCPU
	availableMem := hostMem - platformReservedMemory

	info := map[string]interface{}{
		"cpu_cores":             hostCPU,
		"go_version":            runtime.Version(),
		"os":                    runtime.GOOS,
		"arch":                  runtime.GOARCH,
		"hostname":              readHostHostname(),
		"total_memory":          hostMem,
		"disk":                  readDiskUsage(),
		"platform_reserved_cpu": platformReservedCPU,
		"platform_reserved_mem": platformReservedMemory,
		"available_cpu":         availableCPU,
		"available_memory":      availableMem,
	}

	// Calculate total allocated resources across all apps
	apps, _, err := h.appRepo.ListAll(ctx, 1000, 0)
	if err == nil {
		var totalCPU float64
		var totalMemory int64
		for _, app := range apps {
			if app.ResourceLimits.CPU != "" {
				if cpu, err := strconv.ParseFloat(app.ResourceLimits.CPU, 64); err == nil {
					totalCPU += cpu
				}
			} else {
				totalCPU += 0.5 // default
			}
			if app.ResourceLimits.Memory != "" {
				totalMemory += parseMemoryString(app.ResourceLimits.Memory)
			} else {
				totalMemory += 512 * 1024 * 1024 // default 512MB
			}
		}
		info["allocated_cpu"] = fmt.Sprintf("%.1f", totalCPU)
		info["allocated_memory"] = totalMemory
		info["total_apps_counted"] = len(apps)
		info["free_cpu"] = fmt.Sprintf("%.1f", availableCPU-totalCPU)
		info["free_memory"] = availableMem - totalMemory
	}

	writeJSON(w, http.StatusOK, info)
}

// readHostResources uses "docker info" to get the host's actual CPU and memory.
func readHostResources() (cpuCores int, totalMemory int64) {
	// Default to container values
	cpuCores = runtime.NumCPU()

	out, err := exec.Command("docker", "info", "--format", "{{.NCPU}} {{.MemTotal}}").Output()
	if err != nil {
		// Fallback: read /proc/meminfo for memory
		totalMemory = readProcMemTotal()
		return
	}

	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) >= 2 {
		if n, err := strconv.Atoi(parts[0]); err == nil {
			cpuCores = n
		}
		if m, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
			totalMemory = m
		}
	}
	if totalMemory == 0 {
		totalMemory = readProcMemTotal()
	}
	return
}

// readHostHostname gets the actual host hostname via "docker info".
func readHostHostname() string {
	out, err := exec.Command("docker", "info", "--format", "{{.Name}}").Output()
	if err != nil {
		h, _ := os.Hostname()
		return h
	}
	name := strings.TrimSpace(string(out))
	if name != "" {
		return name
	}
	h, _ := os.Hostname()
	return h
}

// readProcMemTotal reads total system memory from /proc/meminfo as fallback.
func readProcMemTotal() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				kb, _ := strconv.ParseInt(parts[1], 10, 64)
				return kb * 1024 // return bytes
			}
		}
	}
	return 0
}

// readDiskUsage uses df to get disk info for the root partition.
func readDiskUsage() map[string]interface{} {
	out, err := exec.Command("df", "-B1", "/").Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return nil
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 6 {
		return nil
	}
	total, _ := strconv.ParseInt(fields[1], 10, 64)
	used, _ := strconv.ParseInt(fields[2], 10, 64)
	available, _ := strconv.ParseInt(fields[3], 10, 64)
	return map[string]interface{}{
		"total":     total,
		"used":      used,
		"available": available,
		"percent":   fields[4],
	}
}

// parseMemoryString converts memory strings like "512m", "1g" to bytes.
func parseMemoryString(s string) int64 {
	s = strings.TrimSpace(strings.ToLower(s))
	if len(s) == 0 {
		return 0
	}
	suffix := s[len(s)-1]
	numStr := s[:len(s)-1]
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0
	}
	switch suffix {
	case 'g':
		return int64(val * 1024 * 1024 * 1024)
	case 'm':
		return int64(val * 1024 * 1024)
	case 'k':
		return int64(val * 1024)
	default:
		v, _ := strconv.ParseInt(s, 10, 64)
		return v
	}
}
