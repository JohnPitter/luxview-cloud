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

	apps, err := hc.appRepo.ListAllRunning(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to list running apps")
		return
	}

	for _, app := range apps {
		healthy := hc.CheckApp(ctx, &app)
		if !healthy {
			log.Warn().Str("app", app.Subdomain).Msg("app is unhealthy")
			if err := hc.appRepo.UpdateStatus(ctx, app.ID, model.AppStatusError, app.ContainerID); err != nil {
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

	// HTTP health check
	url := fmt.Sprintf("http://host.docker.internal:%d/", app.AssignedPort)
	resp, err := hc.client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode < 500
}

// WaitForHealthy polls health until the app responds or timeout.
func (hc *HealthChecker) WaitForHealthy(ctx context.Context, appID uuid.UUID, port int, timeout time.Duration) bool {
	log := logger.With("healthcheck")
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://host.docker.internal:%d/", port)

	for time.Now().Before(deadline) {
		resp, err := hc.client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				log.Info().Str("app_id", appID.String()).Msg("app is healthy")
				return true
			}
		}

		select {
		case <-ctx.Done():
			return false
		case <-time.After(3 * time.Second):
		}
	}

	log.Warn().Str("app_id", appID.String()).Msg("health check timed out")
	return false
}
