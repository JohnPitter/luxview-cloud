package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/logger"
)

// PushEventService processes a push to a LuxView-hosted repository.
// It mirrors the GitHub webhook flow but uses the internal repository as source.
type PushEventService struct {
	appRepo    *repository.AppRepo
	repoSvc    *RepositoryService
	actionSvc  *ActionService
	buildQueue chan<- DeployRequest
}

func NewPushEventService(appRepo *repository.AppRepo, repoSvc *RepositoryService, actionSvc *ActionService, buildQueue chan<- DeployRequest) *PushEventService {
	return &PushEventService{
		appRepo:    appRepo,
		repoSvc:    repoSvc,
		actionSvc:  actionSvc,
		buildQueue: buildQueue,
	}
}

// HandlePush is called after a git receive-pack completes for the given repository.
// It finds apps that reference the repository and dispatches Actions/deploy for each.
func (s *PushEventService) HandlePush(ctx context.Context, repositoryID uuid.UUID) error {
	log := logger.With("push-event")

	apps, _, err := s.appRepo.ListAll(ctx, 1000, 0)
	if err != nil {
		return fmt.Errorf("list apps: %w", err)
	}

	commitSHA, resolveErr := s.repoSvc.ResolveRef(ctx, repositoryID, "")

	matched := 0
	for _, app := range apps {
		if app.RepositoryID == nil || *app.RepositoryID != repositoryID {
			continue
		}
		if !app.AutoDeploy {
			continue
		}

		sha := ""
		if resolveErr == nil {
			sha = commitSHA
		}

		if s.actionSvc != nil {
			_, err := s.actionSvc.TriggerRun(ctx, app.ID, TriggerActionRequest{
				CommitSHA: sha,
				Trigger:   actionTriggerPush,
			})
			if err == nil {
				log.Info().Str("app_id", app.ID.String()).Msg("action run queued from push")
				matched++
				continue
			}
			if !errors.Is(err, ErrActionWorkflowNotFound) {
				log.Warn().Err(err).Str("app_id", app.ID.String()).Msg("failed to queue action run, falling back to deploy")
			}
		}

		select {
		case s.buildQueue <- DeployRequest{
			AppID:  app.ID,
			UserID: app.UserID,
			Source: "auto",
		}:
			log.Info().Str("app_id", app.ID.String()).Msg("deploy queued from push")
			matched++
		default:
			log.Warn().Str("app_id", app.ID.String()).Msg("build queue full, skipping")
		}
	}

	log.Info().Int("matched", matched).Str("repo_id", repositoryID.String()).Msg("push event processed")

	// Sync backup remotes in background — failure must not block the response.
	go func() {
		// Find the owner of this repository to get the GitHub token.
		apps2, _, _ := s.appRepo.ListAll(context.Background(), 1000, 0)
		for _, app := range apps2 {
			if app.RepositoryID != nil && *app.RepositoryID == repositoryID {
				s.repoSvc.SyncAllBackups(context.Background(), repositoryID, app.UserID)
				return
			}
		}
	}()

	return nil
}
