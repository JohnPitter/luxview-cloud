package worker

import (
	"context"
	"time"

	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

// HealthCheckWorker periodically checks the health of running containers.
type HealthCheckWorker struct {
	checker  *service.HealthChecker
	interval time.Duration
}

func NewHealthCheckWorker(checker *service.HealthChecker, intervalSec int) *HealthCheckWorker {
	return &HealthCheckWorker{
		checker:  checker,
		interval: time.Duration(intervalSec) * time.Second,
	}
}

// Start begins the health check loop.
func (hw *HealthCheckWorker) Start(ctx context.Context) {
	log := logger.With("healthcheck-worker")
	log.Info().Dur("interval", hw.interval).Msg("starting healthcheck worker")

	ticker := time.NewTicker(hw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("healthcheck worker stopped")
			return
		case <-ticker.C:
			hw.checker.CheckAll(ctx)
		}
	}
}
