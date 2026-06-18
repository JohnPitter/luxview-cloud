package model

import (
	"time"

	"github.com/google/uuid"
)

type IssueStatus string

const (
	IssueStatusOpen   IssueStatus = "open"
	IssueStatusClosed IssueStatus = "closed"
)

type Issue struct {
	ID           uuid.UUID   `json:"id"`
	RepositoryID uuid.UUID   `json:"repository_id"`
	AuthorID     uuid.UUID   `json:"author_id"`
	Number       int         `json:"number"`
	Title        string      `json:"title"`
	Body         string      `json:"body"`
	Status       IssueStatus `json:"status"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
	ClosedAt     *time.Time  `json:"closed_at,omitempty"`

	// Populated on demand
	Author   *User          `json:"author,omitempty"`
	Labels   []Label        `json:"labels,omitempty"`
	Comments []IssueComment `json:"comments,omitempty"`
}

type Label struct {
	ID           uuid.UUID `json:"id"`
	RepositoryID uuid.UUID `json:"repository_id"`
	Name         string    `json:"name"`
	Color        string    `json:"color"`
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"created_at"`
}

type IssueComment struct {
	ID        uuid.UUID `json:"id"`
	IssueID   uuid.UUID `json:"issue_id"`
	AuthorID  uuid.UUID `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Author *User `json:"author,omitempty"`
}
