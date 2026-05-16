package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/crypto"
	"github.com/luxview/engine/pkg/logger"
)

type RepoCloner struct {
	userRepo      *repository.UserRepo
	encryptionKey []byte
	logName       string
}

func NewRepoCloner(userRepo *repository.UserRepo, encryptionKey []byte, logName string) *RepoCloner {
	return &RepoCloner{userRepo: userRepo, encryptionKey: encryptionKey, logName: logName}
}

func (c *RepoCloner) Clone(ctx context.Context, app *model.App, destDir string) error {
	log := logger.With(c.logName)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create clone dir: %w", err)
	}

	user, err := c.userRepo.FindByID(ctx, app.UserID)
	if err != nil || user == nil {
		return fmt.Errorf("user not found")
	}

	token := user.GitHubToken
	if token != "" {
		if decrypted, err := crypto.Decrypt(token, c.encryptionKey); err == nil {
			token = decrypted
		}
	}

	cloneURL := app.RepoURL
	maskedURL := app.RepoURL
	if token != "" {
		cloneURL = injectTokenInURL(app.RepoURL, token)
		if len(token) >= 4 {
			maskedURL = injectTokenInURL(app.RepoURL, "****"+token[len(token)-4:])
		} else {
			maskedURL = injectTokenInURL(app.RepoURL, "****")
		}
	}

	log.Debug().Str("clone_url", maskedURL).Str("branch", app.RepoBranch).Str("dest_dir", destDir).Msg("cloning repo")
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", app.RepoBranch, cloneURL, destDir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error().Err(err).Str("output", string(output)).Msg("git clone failed")
		return fmt.Errorf("git clone failed: %s", string(output))
	}

	log.Info().Str("repo", app.RepoURL).Str("branch", app.RepoBranch).Msg("repo cloned")
	return nil
}
