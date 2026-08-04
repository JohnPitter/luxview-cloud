package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/crypto"
	"github.com/luxview/engine/pkg/logger"
)

// GameClientStorageService keeps each game's base client in the storage service
// attached to that app. The global asset is used only as the seed for new apps.
type GameClientStorageService struct {
	serviceRepo   *repository.ServiceRepo
	provisioner   *Provisioner
	encryptionKey []byte
	basePaths     map[string]string
}

func NewGameClientStorageService(
	serviceRepo *repository.ServiceRepo,
	provisioner *Provisioner,
	encryptionKey []byte,
	basePaths map[string]string,
) *GameClientStorageService {
	return &GameClientStorageService{
		serviceRepo:   serviceRepo,
		provisioner:   provisioner,
		encryptionKey: encryptionKey,
		basePaths:     basePaths,
	}
}

// Ensure creates the app storage service when needed and returns the client
// path inside it. Existing app-local clients are preferred over global assets.
func (s *GameClientStorageService) Ensure(ctx context.Context, appID uuid.UUID, templateID string) (string, error) {
	source := strings.TrimSpace(s.basePaths[templateID])
	if source == "" {
		return "", fmt.Errorf("client base is not configured for template %s", templateID)
	}

	svc, err := s.serviceRepo.FindByAppAndType(ctx, appID, model.ServiceStorage)
	if err != nil {
		return "", fmt.Errorf("find game storage: %w", err)
	}
	if svc == nil {
		svc, err = s.provisioner.Provision(ctx, appID, model.ServiceStorage)
		if err != nil {
			return "", fmt.Errorf("provision game storage: %w", err)
		}
	}

	hostPath, err := storageHostPath(svc, s.encryptionKey)
	if err != nil {
		return "", err
	}
	target, err := safeStorageFile(hostPath, filepath.Base(source))
	if err != nil {
		return "", err
	}
	if isRegularFile(target) {
		return target, nil
	}
	if !isRegularFile(source) {
		return "", fmt.Errorf("client base zip not found: %s", source)
	}
	if err := seedClientFile(source, target); err != nil {
		return "", fmt.Errorf("seed client storage: %w", err)
	}

	log := logger.With("game-client-storage")
	log.Info().Str("app_id", appID.String()).Str("template", templateID).Str("path", target).Msg("game client stored in app service")
	return target, nil
}

func storageHostPath(svc *model.AppService, key []byte) (string, error) {
	if svc.CredentialsPlain != nil && svc.CredentialsPlain["host_path"] != "" {
		return svc.CredentialsPlain["host_path"], nil
	}
	var encrypted string
	if err := json.Unmarshal(svc.Credentials, &encrypted); err != nil {
		return "", fmt.Errorf("decode storage credentials: %w", err)
	}
	decrypted, err := crypto.Decrypt(encrypted, key)
	if err != nil {
		return "", fmt.Errorf("decrypt storage credentials: %w", err)
	}
	var creds map[string]string
	if err := json.Unmarshal([]byte(decrypted), &creds); err != nil {
		return "", fmt.Errorf("parse storage credentials: %w", err)
	}
	if creds["host_path"] == "" {
		return "", fmt.Errorf("storage service has no host path")
	}
	return creds["host_path"], nil
}

func safeStorageFile(basePath, name string) (string, error) {
	if name == "" || name == "." || name != filepath.Base(name) {
		return "", fmt.Errorf("invalid client filename")
	}
	base, err := filepath.Abs(basePath)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(base, name))
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(target, base+string(filepath.Separator)) {
		return "", fmt.Errorf("client path escapes storage service")
	}
	return target, nil
}

func seedClientFile(source, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	if err := os.Link(source, target); err == nil {
		return nil
	}

	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(target)
		return err
	}
	return out.Close()
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}
