package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/logger"
)

// healthPaths are the paths to try when checking if a container is healthy.
var healthPaths = []string{"/", "/health", "/api/health"}

// unhealthyThreshold is the number of consecutive failed checks required before
// flipping an app from running to error. Prevents single-tick flakiness from
// showing users a red status during the few seconds after a redeploy when the
// app is technically up but still warming up (DB pool, JVM, lazy routes).
const unhealthyThreshold = 3

// HealthChecker performs health checks on running containers.
type HealthChecker struct {
	appRepo   *repository.AppRepo
	container *ContainerManager
	client    *http.Client

	mu       sync.Mutex
	failures map[uuid.UUID]int
}

// checkOutcome describes the result of a single health probe, carrying the
// reason when unhealthy so CheckAll can log actionable debug info.
type checkOutcome struct {
	healthy bool
	reason  string
	detail  string
}

func NewHealthChecker(appRepo *repository.AppRepo, container *ContainerManager) *HealthChecker {
	return &HealthChecker{
		appRepo:   appRepo,
		container: container,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		failures: make(map[uuid.UUID]int),
	}
}

// CheckAll checks all running apps and updates their status.
func (hc *HealthChecker) CheckAll(ctx context.Context) {
	log := logger.With("healthcheck")

	// Check both running and error apps (error apps may have recovered)
	apps, err := hc.appRepo.ListAllRunningOrError(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to list apps for health check")
		return
	}

	for _, app := range apps {
		if app.Status == model.AppStatusMaintenance || app.Status == model.AppStatusBuilding || app.Status == "deploying" {
			continue
		}
		if app.AppType == model.AppTypeGame {
			outcome, containerID := hc.checkGame(ctx, &app)
			if outcome.healthy {
				hc.resetFailures(app.ID)
				if app.Status == model.AppStatusError || (containerID != "" && containerID != app.ContainerID) {
					log.Info().Str("app", app.Subdomain).Str("container", containerID).Msg("game recovered, marking as running")
					_ = hc.appRepo.UpdateStatus(ctx, app.ID, model.AppStatusRunning, containerID)
				}
				continue
			}
			count := hc.recordFailure(app.ID)
			if app.Status == model.AppStatusRunning && count >= unhealthyThreshold {
				log.Warn().Str("app", app.Subdomain).Str("reason", outcome.reason).Msg("game port unreachable, marking as error")
				_ = hc.appRepo.UpdateStatus(ctx, app.ID, model.AppStatusError, app.ContainerID)
			}
			continue
		}
		outcome := hc.checkApp(ctx, &app)

		if outcome.healthy {
			hc.resetFailures(app.ID)
			if app.Status == model.AppStatusError {
				log.Info().Str("app", app.Subdomain).Msg("app recovered, marking as running")
				if err := hc.appRepo.UpdateStatus(ctx, app.ID, model.AppStatusRunning, app.ContainerID); err != nil {
					log.Error().Err(err).Str("app", app.Subdomain).Msg("failed to update app status")
				}
			}
			continue
		}

		// Unhealthy: always log why (debug), and only flip to error after N consecutive failures.
		count := hc.recordFailure(app.ID)
		log.Debug().
			Str("app", app.Subdomain).
			Str("reason", outcome.reason).
			Str("detail", outcome.detail).
			Int("consecutive_failures", count).
			Int("threshold", unhealthyThreshold).
			Msg("health check failed")

		if app.Status == model.AppStatusRunning && count >= unhealthyThreshold {
			log.Warn().
				Str("app", app.Subdomain).
				Str("reason", outcome.reason).
				Str("detail", outcome.detail).
				Int("consecutive_failures", count).
				Msg("app is unhealthy, marking as error")
			if err := hc.appRepo.UpdateStatus(ctx, app.ID, model.AppStatusError, app.ContainerID); err != nil {
				log.Error().Err(err).Str("app", app.Subdomain).Msg("failed to update app status")
			}
		}
	}
}

// CheckApp performs a health check on a single app. Exported for callers that
// just need a boolean answer.
func (hc *HealthChecker) CheckApp(ctx context.Context, app *model.App) bool {
	return hc.checkApp(ctx, app).healthy
}

func (hc *HealthChecker) checkGame(ctx context.Context, app *model.App) (checkOutcome, string) {
	containerID := hc.resolveRunningContainer(ctx, app)
	if containerID == "" {
		return checkOutcome{reason: "missing_container_id"}, app.ContainerID
	}
	info, err := hc.container.docker.InspectContainer(ctx, containerID)
	if err != nil {
		return checkOutcome{reason: "docker_inspect_error", detail: err.Error()}, containerID
	}
	if info.State == nil || !info.State.Running {
		return checkOutcome{reason: "container_not_running"}, containerID
	}
	tried := false
	for port, bindings := range info.NetworkSettings.Ports {
		if !strings.HasSuffix(string(port), "/tcp") {
			continue
		}
		for _, b := range bindings {
			if b.HostPort == "" {
				continue
			}
			tried = true
			conn, err := net.DialTimeout("tcp", net.JoinHostPort("host.docker.internal", b.HostPort), 2*time.Second)
			if err == nil {
				conn.Close()
				return checkOutcome{healthy: true}, containerID
			}
		}
	}
	for _, nw := range info.NetworkSettings.Networks {
		if nw.IPAddress == "" {
			continue
		}
		for p := range info.Config.ExposedPorts {
			port, _, _ := strings.Cut(string(p), "/")
			if _, err := strconv.Atoi(port); err != nil {
				continue
			}
			tried = true
			conn, err := net.DialTimeout("tcp", net.JoinHostPort(nw.IPAddress, port), 2*time.Second)
			if err == nil {
				conn.Close()
				return checkOutcome{healthy: true}, containerID
			}
		}
		break
	}
	if tried {
		return checkOutcome{reason: "game_port_unreachable"}, containerID
	}
	return checkOutcome{healthy: true}, containerID
}

// resolveRunningContainer prefers the stored ID, then the stable game name
// luxview-game-<subdomain>. Recreating a container (new ID, same name) used to
// mark listed games as error and grey them out in the launcher.
func (hc *HealthChecker) resolveRunningContainer(ctx context.Context, app *model.App) string {
	if id := runningContainerID(hc, ctx, app.ContainerID); id != "" {
		return id
	}
	if app.AppType == model.AppTypeGame {
		if id := runningContainerID(hc, ctx, ContainerName(app.Subdomain)); id != "" {
			return id
		}
	}
	return ""
}

func runningContainerID(hc *HealthChecker, ctx context.Context, ref string) string {
	if ref == "" {
		return ""
	}
	info, err := hc.container.docker.InspectContainer(ctx, ref)
	if err != nil || info.State == nil || !info.State.Running {
		return ""
	}
	if info.ID != "" {
		return info.ID
	}
	return ref
}

func (hc *HealthChecker) checkApp(ctx context.Context, app *model.App) checkOutcome {
	if app.ContainerID == "" {
		return checkOutcome{reason: "missing_container_id"}
	}
	if app.AssignedPort == 0 {
		return checkOutcome{reason: "missing_assigned_port"}
	}

	// Check if container is running via Docker
	running, err := hc.container.IsRunning(ctx, app.ContainerID)
	if err != nil {
		return checkOutcome{reason: "docker_inspect_error", detail: err.Error()}
	}
	if !running {
		return checkOutcome{reason: "container_not_running", detail: app.ContainerID[:min(12, len(app.ContainerID))]}
	}

	// Build base URLs: container IP first (more reliable), then host port
	bases := []string{
		fmt.Sprintf("http://host.docker.internal:%d", app.AssignedPort),
	}
	info, err := hc.container.docker.InspectContainer(ctx, app.ContainerID)
	if err == nil {
		for _, nw := range info.NetworkSettings.Networks {
			if nw.IPAddress != "" {
				bases = append([]string{fmt.Sprintf("http://%s:%d", nw.IPAddress, app.InternalPort)}, bases...)
				break
			}
		}
	}

	// Try each base with /, /health, /api/health. Track the last failure so we
	// can report something meaningful when every probe fails.
	lastReason := "no_response"
	lastDetail := ""
	for _, base := range bases {
		for _, path := range healthPaths {
			url := base + path
			resp, err := hc.client.Get(url)
			if err != nil {
				lastReason = "http_error"
				lastDetail = fmt.Sprintf("%s: %s", url, err.Error())
				continue
			}
			statusCode := resp.StatusCode
			resp.Body.Close()
			if statusCode < 500 {
				return checkOutcome{healthy: true}
			}
			lastReason = "http_5xx"
			lastDetail = fmt.Sprintf("%s: %d", url, statusCode)
		}
	}
	return checkOutcome{reason: lastReason, detail: lastDetail}
}

func (hc *HealthChecker) recordFailure(appID uuid.UUID) int {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.failures[appID]++
	return hc.failures[appID]
}

func (hc *HealthChecker) resetFailures(appID uuid.UUID) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	delete(hc.failures, appID)
}

// WaitForHealthy polls health until the app responds or timeout.
// It tries multiple URLs: container IP on Docker network first, then host.docker.internal.
func (hc *HealthChecker) WaitForHealthy(ctx context.Context, appID uuid.UUID, containerID string, internalPort int, hostPort int, timeout time.Duration) bool {
	log := logger.With("healthcheck")
	deadline := time.Now().Add(timeout)

	// Build base URLs to try (container IP is most reliable)
	bases := []string{
		fmt.Sprintf("http://host.docker.internal:%d", hostPort),
	}

	// Try to get container IP for direct health check (more reliable than host routing)
	if containerID != "" {
		info, err := hc.container.docker.InspectContainer(ctx, containerID)
		if err == nil {
			for _, nw := range info.NetworkSettings.Networks {
				if nw.IPAddress != "" {
					bases = append([]string{fmt.Sprintf("http://%s:%d", nw.IPAddress, internalPort)}, bases...)
					break
				}
			}
		}
	}

	log.Debug().Strs("bases", bases).Strs("paths", healthPaths).Str("app_id", appID.String()).Dur("timeout", timeout).Msg("starting health check")

	attempt := 0
	for time.Now().Before(deadline) {
		attempt++
		for _, base := range bases {
			for _, path := range healthPaths {
				url := base + path
				resp, err := hc.client.Get(url)
				if err != nil {
					log.Debug().Str("app_id", appID.String()).Int("attempt", attempt).Str("url", url).Err(err).Msg("health check attempt failed")
				} else {
					statusCode := resp.StatusCode
					resp.Body.Close()
					log.Debug().Str("app_id", appID.String()).Int("attempt", attempt).Str("url", url).Int("status_code", statusCode).Msg("health check attempt result")
					if statusCode < 500 {
						log.Info().Str("app_id", appID.String()).Str("url", url).Int("attempts", attempt).Msg("app is healthy")
						return true
					}
				}
			}
		}

		select {
		case <-ctx.Done():
			log.Debug().Str("app_id", appID.String()).Int("total_attempts", attempt).Msg("health check context cancelled")
			return false
		case <-time.After(3 * time.Second):
		}
	}

	log.Warn().Str("app_id", appID.String()).Int("total_attempts", attempt).Msg("health check timed out")
	return false
}
