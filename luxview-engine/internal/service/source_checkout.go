package service

import (
	"context"
	"fmt"

	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
)

type SourceCheckout interface {
	Checkout(ctx context.Context, app *model.App, ref string, destDir string) (*model.CheckoutResult, error)
}

type GitHubSourceCheckout struct {
	cloner *RepoCloner
}

func NewGitHubSourceCheckout(userRepo *repository.UserRepo, encryptionKey []byte, logName string) *GitHubSourceCheckout {
	return &GitHubSourceCheckout{cloner: NewRepoCloner(userRepo, encryptionKey, logName)}
}

func (c *GitHubSourceCheckout) Checkout(ctx context.Context, app *model.App, ref string, destDir string) (*model.CheckoutResult, error) {
	if err := c.cloner.Clone(ctx, app, destDir); err != nil {
		return nil, err
	}
	if ref == "" {
		ref = app.RepoBranch
	}
	return &model.CheckoutResult{
		Ref:       ref,
		CommitSHA: "",
		WorkDir:   destDir,
	}, nil
}

type LuxViewSourceCheckout struct {
	repositorySvc *RepositoryService
}

func NewLuxViewSourceCheckout(repositorySvc *RepositoryService) *LuxViewSourceCheckout {
	return &LuxViewSourceCheckout{repositorySvc: repositorySvc}
}

func (c *LuxViewSourceCheckout) Checkout(ctx context.Context, app *model.App, ref string, destDir string) (*model.CheckoutResult, error) {
	if app.RepositoryID == nil {
		return nil, fmt.Errorf("repository_id is required")
	}
	if ref == "" {
		ref = app.RepoBranch
	}
	return c.repositorySvc.Checkout(ctx, *app.RepositoryID, ref, destDir)
}

type AppSourceCheckout struct {
	legacy  SourceCheckout
	luxview SourceCheckout
}

func NewAppSourceCheckout(legacy, luxview SourceCheckout) *AppSourceCheckout {
	return &AppSourceCheckout{legacy: legacy, luxview: luxview}
}

func (c *AppSourceCheckout) Checkout(ctx context.Context, app *model.App, ref string, destDir string) (*model.CheckoutResult, error) {
	if app.RepositoryID != nil {
		if c.luxview == nil {
			return nil, fmt.Errorf("luxview repository checkout is not configured")
		}
		return c.luxview.Checkout(ctx, app, ref, destDir)
	}
	if c.legacy == nil {
		return nil, fmt.Errorf("legacy repository checkout is not configured")
	}
	return c.legacy.Checkout(ctx, app, ref, destDir)
}
