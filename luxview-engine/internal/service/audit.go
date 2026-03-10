package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/logger"
)

type AuditService struct {
	repo *repository.AuditLogRepo
}

func NewAuditService(repo *repository.AuditLogRepo) *AuditService {
	return &AuditService{repo: repo}
}

type AuditEntry struct {
	ActorID      uuid.UUID
	ActorUsername string
	Action       string
	ResourceType string
	ResourceID   string
	ResourceName string
	OldValues    any
	NewValues    any
	IPAddress    string
}

func (s *AuditService) Log(ctx context.Context, entry AuditEntry) {
	go func() {
		log := logger.With("audit")
		actorID := &entry.ActorID
		if entry.ActorID == uuid.Nil {
			actorID = nil
		}

		err := s.repo.Create(context.Background(), &repository.AuditLog{
			ActorID:      actorID,
			ActorUsername: entry.ActorUsername,
			Action:       entry.Action,
			ResourceType: entry.ResourceType,
			ResourceID:   entry.ResourceID,
			ResourceName: entry.ResourceName,
			OldValues:    entry.OldValues,
			NewValues:    entry.NewValues,
			IPAddress:    entry.IPAddress,
		})
		if err != nil {
			log.Error().Err(err).
				Str("action", entry.Action).
				Str("resource", entry.ResourceType).
				Str("actor", entry.ActorUsername).
				Msg("failed to write audit log")
		}
	}()
}
