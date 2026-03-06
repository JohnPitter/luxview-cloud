package worker

import (
	"context"
	"time"

	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

// AlertWorker periodically evaluates alert rules.
type AlertWorker struct {
	alerter  *service.Alerter
	interval time.Duration
}

func NewAlertWorker(alerter *service.Alerter, intervalSec int) *AlertWorker {
	return &AlertWorker{
		alerter:  alerter,
		interval: time.Duration(intervalSec) * time.Second,
	}
}

// Start begins the alert evaluation loop.
func (aw *AlertWorker) Start(ctx context.Context) {
	log := logger.With("alert-worker")
	log.Info().Dur("interval", aw.interval).Msg("starting alert worker")

	ticker := time.NewTicker(aw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("alert worker stopped")
			return
		case <-ticker.C:
			aw.alerter.EvaluateAll(ctx)
		}
	}
}
