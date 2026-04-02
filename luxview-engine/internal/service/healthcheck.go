package service

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/logger"
)

// healthPaths are the paths to try when checking if a container is healthy.
var healthPaths = []string{"/", "/health", "/api/health"}

// HealthChecker performs health checks on running containers.
type HealthChecker struct {
	appRepo   *repository.AppRepo
	container *ContainerManager
	client    *http.Client
}

func NewHealthChecker(appRepo *repository.AppRepo, container *ContainerManager) *HealthChecker {
	return &HealthChecker{
		appRepo:   appRepo,
		container: container,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
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
		// Skip health checks for maintenance/building/deploying apps
		if app.Status == model.AppStatusMaintenance || app.Status == model.AppStatusBuilding || app.Status == "deploying" {
			continue
		}
		healthy := hc.CheckApp(ctx, &app)
		if !healthy && app.Status == model.AppStatusRunning {
			log.Warn().Str("app", app.Subdomain).Msg("app is unhealthy")
			if err := hc.appRepo.UpdateStatus(ctx, app.ID, model.AppStatusError, app.ContainerID); err != nil {
				log.Error().Err(err).Str("app", app.Subdomain).Msg("failed to update app status")
			}
		} else if healthy && app.Status == model.AppStatusError {
			log.Info().Str("app", app.Subdomain).Msg("app recovered, marking as running")
			if err := hc.appRepo.UpdateStatus(ctx, app.ID, model.AppStatusRunning, app.ContainerID); err != nil {
				log.Error().Err(err).Str("app", app.Subdomain).Msg("failed to update app status")
			}
		}
	}
}

// CheckApp performs a health check on a single app.
func (hc *HealthChecker) CheckApp(ctx context.Context, app *model.App) bool {
	if app.ContainerID == "" || app.AssignedPort == 0 {
		return false
	}

	// Check if container is running via Docker
	running, err := hc.container.IsRunning(ctx, app.ContainerID)
	if err != nil || !running {
		return false
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

	// Try each base with /, /health, /api/health
	for _, base := range bases {
		for _, path := range healthPaths {
			resp, err := hc.client.Get(base + path)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode < 500 {
					return true
				}
			}
		}
	}
	return false
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
