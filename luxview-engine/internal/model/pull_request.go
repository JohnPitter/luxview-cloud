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

type ReviewState string

const (
	ReviewStateApproved         ReviewState = "approved"
	ReviewStateChangesRequested ReviewState = "changes_requested"
	ReviewStateCommented        ReviewState = "commented"
)

type PullRequestReview struct {
	ID            uuid.UUID   `json:"id"`
	PullRequestID uuid.UUID   `json:"pull_request_id"`
	ReviewerID    uuid.UUID   `json:"reviewer_id"`
	State         ReviewState `json:"state"`
	Body          string      `json:"body"`
	CommitSHA     string      `json:"commit_sha"`
	CreatedAt     time.Time   `json:"created_at"`

	Reviewer *User `json:"reviewer,omitempty"`
}

type ReviewSide string

const (
	ReviewSideOld ReviewSide = "old"
	ReviewSideNew ReviewSide = "new"
)

type ReviewComment struct {
	ID            uuid.UUID  `json:"id"`
	PullRequestID uuid.UUID  `json:"pull_request_id"`
	AuthorID      uuid.UUID  `json:"author_id"`
	Path          string     `json:"path"`
	Line          int        `json:"line"`
	Side          ReviewSide `json:"side"`
	Body          string     `json:"body"`
	Resolved      bool       `json:"resolved"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`

	Author *User `json:"author,omitempty"`
}

// MergeStrategy controls how a pull request is integrated into its base branch.
type MergeStrategy string

const (
	MergeStrategyMerge  MergeStrategy = "merge"
	MergeStrategySquash MergeStrategy = "squash"
	MergeStrategyRebase MergeStrategy = "rebase"
)

// StatusCheck reflects a CI run (action_run) associated with a PR head commit.
type StatusCheck struct {
	Name      string     `json:"name"`
	Status    string     `json:"status"`
	RunID     uuid.UUID  `json:"run_id"`
	CommitSHA string     `json:"commit_sha"`
	CreatedAt time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}
