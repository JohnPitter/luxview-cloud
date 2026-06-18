package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
)

type IssueStore interface {
	NextNumber(ctx context.Context, repositoryID uuid.UUID) (int, error)
	Create(ctx context.Context, i *model.Issue) error
	FindByNumber(ctx context.Context, repositoryID uuid.UUID, number int) (*model.Issue, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Issue, error)
	List(ctx context.Context, repositoryID uuid.UUID, status string, limit, offset int) ([]model.Issue, int, error)
	Update(ctx context.Context, id uuid.UUID, title, body string) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.IssueStatus) error
	CreateLabel(ctx context.Context, l *model.Label) error
	ListLabels(ctx context.Context, repositoryID uuid.UUID) ([]model.Label, error)
	DeleteLabel(ctx context.Context, repositoryID, labelID uuid.UUID) error
	SetIssueLabels(ctx context.Context, issueID uuid.UUID, labelIDs []uuid.UUID) error
	LabelsForIssue(ctx context.Context, issueID uuid.UUID) ([]model.Label, error)
	CreateComment(ctx context.Context, c *model.IssueComment) error
	ListComments(ctx context.Context, issueID uuid.UUID) ([]model.IssueComment, error)
	DeleteComment(ctx context.Context, commentID, authorID uuid.UUID) error
}

type IssueService struct {
	store IssueStore
}

func NewIssueService(store IssueStore) *IssueService {
	return &IssueService{store: store}
}

type CreateIssueRequest struct {
	RepositoryID uuid.UUID
	AuthorID     uuid.UUID
	Title        string
	Body         string
	LabelIDs     []uuid.UUID
}

func (s *IssueService) Create(ctx context.Context, req CreateIssueRequest) (*model.Issue, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	number, err := s.store.NextNumber(ctx, req.RepositoryID)
	if err != nil {
		return nil, err
	}
	issue := &model.Issue{
		RepositoryID: req.RepositoryID,
		AuthorID:     req.AuthorID,
		Number:       number,
		Title:        title,
		Body:         strings.TrimSpace(req.Body),
		Status:       model.IssueStatusOpen,
	}
	if err := s.store.Create(ctx, issue); err != nil {
		return nil, err
	}
	if len(req.LabelIDs) > 0 {
		if err := s.store.SetIssueLabels(ctx, issue.ID, req.LabelIDs); err != nil {
			return nil, err
		}
	}
	return s.Get(ctx, req.RepositoryID, number)
}

func (s *IssueService) Get(ctx context.Context, repositoryID uuid.UUID, number int) (*model.Issue, error) {
	issue, err := s.store.FindByNumber(ctx, repositoryID, number)
	if err != nil {
		return nil, err
	}
	if issue == nil {
		return nil, fmt.Errorf("issue not found")
	}
	labels, err := s.store.LabelsForIssue(ctx, issue.ID)
	if err != nil {
		return nil, err
	}
	issue.Labels = labels
	return issue, nil
}

func (s *IssueService) List(ctx context.Context, repositoryID uuid.UUID, status string, limit, offset int) ([]model.Issue, int, error) {
	issues, total, err := s.store.List(ctx, repositoryID, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	for i := range issues {
		labels, err := s.store.LabelsForIssue(ctx, issues[i].ID)
		if err != nil {
			return nil, 0, err
		}
		issues[i].Labels = labels
	}
	return issues, total, nil
}

func (s *IssueService) Update(ctx context.Context, repositoryID uuid.UUID, number int, title, body string, labelIDs []uuid.UUID, setLabels bool) (*model.Issue, error) {
	issue, err := s.store.FindByNumber(ctx, repositoryID, number)
	if err != nil {
		return nil, err
	}
	if issue == nil {
		return nil, fmt.Errorf("issue not found")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = issue.Title
	}
	if err := s.store.Update(ctx, issue.ID, title, strings.TrimSpace(body)); err != nil {
		return nil, err
	}
	if setLabels {
		if err := s.store.SetIssueLabels(ctx, issue.ID, labelIDs); err != nil {
			return nil, err
		}
	}
	return s.Get(ctx, repositoryID, number)
}

func (s *IssueService) SetStatus(ctx context.Context, repositoryID uuid.UUID, number int, status model.IssueStatus) (*model.Issue, error) {
	issue, err := s.store.FindByNumber(ctx, repositoryID, number)
	if err != nil {
		return nil, err
	}
	if issue == nil {
		return nil, fmt.Errorf("issue not found")
	}
	if err := s.store.UpdateStatus(ctx, issue.ID, status); err != nil {
		return nil, err
	}
	return s.Get(ctx, repositoryID, number)
}

// Labels

func (s *IssueService) CreateLabel(ctx context.Context, repositoryID uuid.UUID, name, color, description string) (*model.Label, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("label name is required")
	}
	if color == "" {
		color = "#6366f1"
	}
	label := &model.Label{
		RepositoryID: repositoryID,
		Name:         name,
		Color:        color,
		Description:  strings.TrimSpace(description),
	}
	if err := s.store.CreateLabel(ctx, label); err != nil {
		return nil, err
	}
	return label, nil
}

func (s *IssueService) ListLabels(ctx context.Context, repositoryID uuid.UUID) ([]model.Label, error) {
	return s.store.ListLabels(ctx, repositoryID)
}

func (s *IssueService) DeleteLabel(ctx context.Context, repositoryID, labelID uuid.UUID) error {
	return s.store.DeleteLabel(ctx, repositoryID, labelID)
}

// Comments

func (s *IssueService) AddComment(ctx context.Context, issueID, authorID uuid.UUID, body string) (*model.IssueComment, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("comment body is required")
	}
	c := &model.IssueComment{IssueID: issueID, AuthorID: authorID, Body: body}
	if err := s.store.CreateComment(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *IssueService) ListComments(ctx context.Context, issueID uuid.UUID) ([]model.IssueComment, error) {
	return s.store.ListComments(ctx, issueID)
}

func (s *IssueService) DeleteComment(ctx context.Context, commentID, authorID uuid.UUID) error {
	return s.store.DeleteComment(ctx, commentID, authorID)
}
