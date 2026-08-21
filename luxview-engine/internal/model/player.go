package model

import (
	"time"

	"github.com/google/uuid"
)

type LedgerKind string

const (
	LedgerCash   LedgerKind = "cash"
	LedgerReward LedgerKind = "reward"
)

type PlayerAccount struct {
	ID           uuid.UUID `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CashPoints   int64     `json:"cash_points"`
	RewardPoints int64     `json:"reward_points"`
	CreatedAt    time.Time `json:"created_at"`
}

type PlayerGameLink struct {
	ID          uuid.UUID `json:"id"`
	PlayerID    uuid.UUID `json:"player_id"`
	AppID       uuid.UUID `json:"app_id"`
	TemplateID  string    `json:"template_id"`
	InGameNick  string    `json:"in_game_nick"`
	CreatedAt   time.Time `json:"created_at"`
}

type PlayerLedger struct {
	ID        uuid.UUID  `json:"id"`
	PlayerID  uuid.UUID  `json:"player_id"`
	Kind      LedgerKind `json:"kind"`
	Delta     int64      `json:"delta"`
	Reason    string     `json:"reason"`
	CreatedAt time.Time  `json:"created_at"`
}

type PlayerPublic struct {
	ID           uuid.UUID        `json:"id"`
	Username     string           `json:"username"`
	CashPoints   int64            `json:"cash_points"`
	RewardPoints int64            `json:"reward_points"`
	Links        []PlayerGameLink `json:"links"`
}
