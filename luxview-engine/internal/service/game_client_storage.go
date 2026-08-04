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

// GameClientStorageService resolves a client from global storage and keeps
// compatibility with app-local client services created by older versions.
type GameClientStorageService struct {
	serviceRepo   *repository.ServiceRepo
	encryptionKey []byte
	globalRoot    string
	basePaths     map[string]string
}

func NewGameClientStorageService(
	serviceRepo *repository.ServiceRepo,
	encryptionKey []byte,
	globalRoot string,
	basePaths map[string]string,
) *GameClientStorageService {
	return &GameClientStorageService{
		serviceRepo:   serviceRepo,
		encryptionKey: encryptionKey,
		globalRoot:    globalRoot,
		basePaths:     basePaths,
	}
}

// Resolve returns the configured global client or the legacy app-local file.
// A missing app-local service falls back to the template's global default and
// never provisions a storage service just for the client download.
func (s *GameClientStorageService) Resolve(ctx context.Context, appID uuid.UUID, templateID, globalKey string) (string, error) {
	if key := strings.TrimSpace(globalKey); key != "" {
		return s.resolveGlobalFile(key)
	}

	if s.serviceRepo != nil {
		localPath, err := s.resolveExistingAppFile(ctx, appID, templateID)
		if err != nil {
			return "", err
		}
		if localPath != "" {
			return localPath, nil
		}
	}

	return s.defaultSource(templateID)
}

func (s *GameClientStorageService) resolveExistingAppFile(ctx context.Context, appID uuid.UUID, templateID string) (string, error) {
	svc, err := s.serviceRepo.FindByAppAndType(ctx, appID, model.ServiceStorage)
	if err != nil {
		return "", fmt.Errorf("find game storage: %w", err)
	}
	if svc == nil {
		return "", nil
	}

	hostPath, err := storageHostPath(svc, s.encryptionKey)
	if err != nil {
		return "", err
	}
	source, sourceErr := s.defaultSource(templateID)
	if sourceErr != nil {
		return "", sourceErr
	}
	target, err := safeStorageFile(hostPath, filepath.Base(source))
	if err != nil {
		return "", err
	}
	if isRegularFile(target) {
		return target, nil
	}
	if err := seedClientFile(source, target); err != nil {
		return "", fmt.Errorf("seed legacy app client storage: %w", err)
	}

	log := logger.With("game-client-storage")
	log.Info().Str("app_id", appID.String()).Str("template", templateID).Str("path", target).Msg("legacy game client restored in app service")
	return target, nil
}

func (s *GameClientStorageService) resolveGlobalFile(key string) (string, error) {
	path, err := safeGlobalPath(s.globalRoot, key)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("global client file not found: %s", key)
	}
	base, err := filepath.EvalSymlinks(s.globalRoot)
	if err != nil || !strings.HasPrefix(resolved, base+string(filepath.Separator)) {
		return "", fmt.Errorf("global client file escapes storage root")
	}
	if !isRegularFile(resolved) {
		return "", fmt.Errorf("global client file not found: %s", key)
	}
	return resolved, nil
}

func (s *GameClientStorageService) defaultSource(templateID string) (string, error) {
	source := strings.TrimSpace(s.basePaths[templateID])
	if source == "" {
		return "", fmt.Errorf("client base is not configured for template %s", templateID)
	}
	if !isRegularFile(source) {
		return "", fmt.Errorf("client base zip not found: %s", source)
	}
	return source, nil
}

// DefaultGlobalKey maps the configured template fallback to the global-root
// relative key shown in the game settings selector.
func (s *GameClientStorageService) DefaultGlobalKey(templateID string) string {
	source := strings.TrimSpace(s.basePaths[templateID])
	if source == "" || s.globalRoot == "" {
		return ""
	}
	relative, err := filepath.Rel(s.globalRoot, source)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return ""
	}
	return filepath.ToSlash(relative)
}

// ListGlobalFiles returns ZIP references without exposing absolute server paths.
func (s *GameClientStorageService) ListGlobalFiles() ([]model.SelectOptionDef, error) {
	if strings.TrimSpace(s.globalRoot) == "" {
		return []model.SelectOptionDef{}, nil
	}
	if _, err := os.Stat(s.globalRoot); err != nil {
		if os.IsNotExist(err) {
			return []model.SelectOptionDef{}, nil
		}
		return nil, err
	}

	options := make([]model.SelectOptionDef, 0)
	err := filepath.WalkDir(s.globalRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(entry.Name()), ".zip") {
			return nil
		}
		relative, err := filepath.Rel(s.globalRoot, path)
		if err != nil {
			return err
		}
		options = append(options, model.SelectOptionDef{
			Value: filepath.ToSlash(relative),
			Label: filepath.ToSlash(relative),
		})
		return nil
	})
	return options, err
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

func safeGlobalPath(root, key string) (string, error) {
	key = strings.TrimSpace(key)
	relative := filepath.FromSlash(key)
	clean := filepath.Clean(relative)
	if key == "" || filepath.IsAbs(relative) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid global storage reference")
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(base, clean))
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(target, base+string(filepath.Separator)) {
		return "", fmt.Errorf("global storage reference escapes root")
	}
	return target, nil
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
	if info, err := os.Lstat(target); err == nil {
		if info.IsDir() {
			return fmt.Errorf("client target is a directory")
		}
		if err := os.Remove(target); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
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
