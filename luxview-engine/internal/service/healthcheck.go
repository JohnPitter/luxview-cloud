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

	// Try container IP first (more reliable), then host port
	urls := []string{
		fmt.Sprintf("http://host.docker.internal:%d/", app.AssignedPort),
	}
	info, err := hc.container.docker.InspectContainer(ctx, app.ContainerID)
	if err == nil {
		for _, nw := range info.NetworkSettings.Networks {
			if nw.IPAddress != "" {
				urls = append([]string{fmt.Sprintf("http://%s:%d/", nw.IPAddress, app.InternalPort)}, urls...)
				break
			}
		}
	}

	for _, url := range urls {
		resp, err := hc.client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return true
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

	// Build list of health check URLs to try (container IP is most reliable)
	urls := []string{
		fmt.Sprintf("http://host.docker.internal:%d/", hostPort),
	}

	// Try to get container IP for direct health check (more reliable than host routing)
	if containerID != "" {
		info, err := hc.container.docker.InspectContainer(ctx, containerID)
		if err == nil {
			for _, nw := range info.NetworkSettings.Networks {
				if nw.IPAddress != "" {
					urls = append([]string{fmt.Sprintf("http://%s:%d/", nw.IPAddress, internalPort)}, urls...)
					break
				}
			}
		}
	}

	log.Debug().Strs("urls", urls).Str("app_id", appID.String()).Dur("timeout", timeout).Msg("starting health check")

	for time.Now().Before(deadline) {
		for _, url := range urls {
			resp, err := hc.client.Get(url)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode < 500 {
					log.Info().Str("app_id", appID.String()).Str("url", url).Msg("app is healthy")
					return true
				}
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
