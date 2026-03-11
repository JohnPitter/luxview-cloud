package worker

import (
	"context"
	"sync"
	"time"

	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

// BuildWorker processes deploy requests from a queue using a goroutine pool.
type BuildWorker struct {
	deployer    *service.Deployer
	queue       chan service.DeployRequest
	concurrency int
	wg          sync.WaitGroup
}

func NewBuildWorker(deployer *service.Deployer, concurrency int) (*BuildWorker, chan service.DeployRequest) {
	queue := make(chan service.DeployRequest, 100)
	return &BuildWorker{
		deployer:    deployer,
		queue:       queue,
		concurrency: concurrency,
	}, queue
}

// Start launches the worker goroutines.
func (bw *BuildWorker) Start(ctx context.Context) {
	log := logger.With("build-worker")
	log.Info().Int("concurrency", bw.concurrency).Msg("starting build worker pool")

	for i := 0; i < bw.concurrency; i++ {
		bw.wg.Add(1)
		go func(workerID int) {
			defer bw.wg.Done()
			wlog := log.With().Int("worker", workerID).Logger()

			for {
				select {
				case <-ctx.Done():
					wlog.Info().Msg("worker shutting down")
					return
				case req, ok := <-bw.queue:
					if !ok {
						wlog.Info().Msg("queue closed, worker exiting")
						return
					}
					queueDepth := len(bw.queue)
					wlog.Info().
						Str("app_id", req.AppID.String()).
						Str("commit", req.CommitSHA).
						Int("queue_depth", queueDepth).
						Msg("processing build")

					deployStart := time.Now()
					deployErr := bw.deployer.Deploy(ctx, req)
					deployDuration := time.Since(deployStart)
					if deployErr != nil {
						wlog.Error().Err(deployErr).
							Str("app_id", req.AppID.String()).
							Dur("duration", deployDuration).
							Msg("deploy failed")
					} else {
						wlog.Info().
							Str("app_id", req.AppID.String()).
							Dur("duration", deployDuration).
							Msg("deploy completed")
					}
				}
			}
		}(i)
	}
}

// Stop waits for all workers to finish.
func (bw *BuildWorker) Stop() {
	close(bw.queue)
	bw.wg.Wait()
}

// Queue returns the build queue channel for enqueueing requests.
func (bw *BuildWorker) Queue() chan service.DeployRequest {
	return bw.queue
}
