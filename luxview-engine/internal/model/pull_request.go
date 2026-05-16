package model

import (
	"time"

	"github.com/google/uuid"
)

type PullRequestStatus string

const (
	PullRequestStatusOpen   PullRequestStatus = "open"
	PullRequestStatusMerged PullRequestStatus = "merged"
	PullRequestStatusClosed PullRequestStatus = "closed"
)

type PullRequest struct {
	ID           uuid.UUID         `json:"id"`
	RepositoryID uuid.UUID         `json:"repository_id"`
	AuthorID     uuid.UUID         `json:"author_id"`
	Number       int               `json:"number"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	HeadBranch   string            `json:"head_branch"`
	BaseBranch   string            `json:"base_branch"`
	HeadSHA      string            `json:"head_sha"`
	Status       PullRequestStatus `json:"status"`
	MergeCommit  *string           `json:"merge_commit,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	MergedAt     *time.Time        `json:"merged_at,omitempty"`
	ClosedAt     *time.Time        `json:"closed_at,omitempty"`

	// Populated on demand
	Author   *User                `json:"author,omitempty"`
	Comments []PullRequestComment `json:"comments,omitempty"`
}

type PullRequestComment struct {
	ID            uuid.UUID `json:"id"`
	PullRequestID uuid.UUID `json:"pull_request_id"`
	AuthorID      uuid.UUID `json:"author_id"`
	Body          string    `json:"body"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	Author *User `json:"author,omitempty"`
}

type PRCommit struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Date    string `json:"date"`
}

type PRFileDiff struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch"`
}
