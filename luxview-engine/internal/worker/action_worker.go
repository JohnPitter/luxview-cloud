package worker

import (
	"context"
	"sync"
	"time"

	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

const actionWorkerPollInterval = 3 * time.Second

type ActionWorker struct {
	actionRepo  *repository.ActionRepo
	actionSvc   *service.ActionService
	concurrency int
	wg          sync.WaitGroup
}

func NewActionWorker(actionRepo *repository.ActionRepo, actionSvc *service.ActionService, concurrency int) *ActionWorker {
	if concurrency <= 0 {
		concurrency = 1
	}
	return &ActionWorker{actionRepo: actionRepo, actionSvc: actionSvc, concurrency: concurrency}
}

func (w *ActionWorker) Start(ctx context.Context) {
	log := logger.With("action-worker")
	log.Info().Int("concurrency", w.concurrency).Msg("starting action worker pool")

	for i := 0; i < w.concurrency; i++ {
		w.wg.Add(1)
		go func(workerID int) {
			defer w.wg.Done()
			wlog := log.With().Int("worker", workerID).Logger()
			ticker := time.NewTicker(actionWorkerPollInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					wlog.Info().Msg("worker shutting down")
					return
				case <-ticker.C:
					run, err := w.actionRepo.ClaimNextQueuedRun(ctx)
					if err != nil {
						wlog.Error().Err(err).Msg("failed to claim action run")
						continue
					}
					if run == nil {
						continue
					}
					wlog.Info().Str("run_id", run.ID.String()).Str("workflow", run.Workflow).Msg("processing action run")
					if err := w.actionSvc.ExecuteRun(ctx, run); err != nil {
						wlog.Error().Err(err).Str("run_id", run.ID.String()).Msg("action run failed")
					}
				}
			}
		}(i)
	}
}

func (w *ActionWorker) Stop() {
	w.wg.Wait()
}
