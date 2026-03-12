package model

import (
	"time"

	"github.com/google/uuid"
)

type Mailbox struct {
	ID        uuid.UUID `json:"id"`
	ServiceID uuid.UUID `json:"service_id"`
	Address   string    `json:"address"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateMailboxRequest struct {
	LocalPart string `json:"local_part"`
}
