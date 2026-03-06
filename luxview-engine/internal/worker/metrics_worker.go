package worker

import (
	"context"
	"time"

	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

// MetricsWorker periodically collects container metrics.
type MetricsWorker struct {
	collector *service.MetricsCollector
	interval  time.Duration
}

func NewMetricsWorker(collector *service.MetricsCollector, intervalSec int) *MetricsWorker {
	return &MetricsWorker{
		collector: collector,
		interval:  time.Duration(intervalSec) * time.Second,
	}
}

// Start begins the metrics collection loop.
func (mw *MetricsWorker) Start(ctx context.Context) {
	log := logger.With("metrics-worker")
	log.Info().Dur("interval", mw.interval).Msg("starting metrics worker")

	ticker := time.NewTicker(mw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("metrics worker stopped")
			return
		case <-ticker.C:
			mw.collector.CollectAll(ctx)
		}
	}
}
