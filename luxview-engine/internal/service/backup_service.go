package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/logger"
)

// ContainerConfig holds container names and credentials needed for backup/restore.
type ContainerConfig struct {
	PGPlatformContainer string
	PGPlatformUser      string
	PGSharedContainer   string
	PGSharedUser        string
	MongoContainer      string
	MongoUser           string
	MongoPassword       string
	RedisContainer      string
	RedisPassword       string
}

type BackupService struct {
	repo         *repository.BackupRepo
	settingsRepo *repository.SettingsRepo
	auditSvc     *AuditService
	backupDir    string
	containers   ContainerConfig
	mu           sync.Mutex
	running      bool
}

func NewBackupService(
	repo *repository.BackupRepo,
	settingsRepo *repository.SettingsRepo,
	auditSvc *AuditService,
	backupDir string,
	containers ContainerConfig,
) *BackupService {
	return &BackupService{
		repo:         repo,
		settingsRepo: settingsRepo,
		auditSvc:     auditSvc,
		backupDir:    backupDir,
		containers:   containers,
	}
}

func (s *BackupService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *BackupService) Run(ctx context.Context, databases []string, trigger model.BackupTrigger, userID *uuid.UUID) (*model.Backup, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil, fmt.Errorf("a backup or restore operation is already running")
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	log := logger.With("backup")
	start := time.Now()

	backup := &model.Backup{
		Databases: databases,
		Status:    model.BackupStatusRunning,
		Trigger:   trigger,
		CreatedBy: userID,
	}

	dirName := buildBackupDirName(start, trigger)
	backupPath := filepath.Join(s.backupDir, dirName)
	backup.FilePath = backupPath

	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return nil, fmt.Errorf("create backup directory: %w", err)
	}

	if err := s.repo.Create(ctx, backup); err != nil {
		return nil, fmt.Errorf("create backup record: %w", err)
	}

	log.Info().Str("id", backup.ID.String()).Strs("databases", databases).Msg("backup started")

	var totalSize int64
	var backupErr error

	for _, db := range databases {
		log.Info().Str("database", db).Msg("backing up database")
		if err := execDump(db, backupPath, s.containers); err != nil {
			backupErr = fmt.Errorf("backup %s failed: %w", db, err)
			log.Error().Err(backupErr).Str("database", db).Msg("database backup failed")
			break
		}
		log.Info().Str("database", db).Msg("database backup completed")
	}

	totalSize = dirSize(backupPath)
	durationMs := int(time.Since(start).Milliseconds())

	meta := map[string]interface{}{
		"databases":   databases,
		"duration_ms": durationMs,
		"file_size":   totalSize,
		"started_at":  start.Format(time.RFC3339),
		"trigger":     string(trigger),
	}
	if metaBytes, err := json.MarshalIndent(meta, "", "  "); err == nil {
		os.WriteFile(filepath.Join(backupPath, "metadata.json"), metaBytes, 0644)
	}

	status := model.BackupStatusCompleted
	errMsg := ""
	if backupErr != nil {
		status = model.BackupStatusFailed
		errMsg = backupErr.Error()
	}

	if err := s.repo.UpdateStatus(ctx, backup.ID, status, errMsg, totalSize, durationMs); err != nil {
		log.Error().Err(err).Msg("failed to update backup status")
	}

	backup.Status = status
	backup.FileSize = totalSize
	backup.DurationMs = durationMs
	backup.Error = errMsg

	log.Info().
		Str("id", backup.ID.String()).
		Str("status", string(status)).
		Int("duration_ms", durationMs).
		Int64("size", totalSize).
		Msg("backup finished")

	return backup, backupErr
}

func (s *BackupService) Restore(ctx context.Context, backupID uuid.UUID) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("a backup or restore operation is already running")
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	log := logger.With("backup")

	backup, err := s.repo.FindByID(ctx, backupID)
	if err != nil {
		return fmt.Errorf("find backup: %w", err)
	}
	if backup == nil {
		return fmt.Errorf("backup not found")
	}
	if backup.Status != model.BackupStatusCompleted {
		return fmt.Errorf("cannot restore backup with status %s", backup.Status)
	}

	log.Info().Str("id", backupID.String()).Strs("databases", backup.Databases).Msg("restore started")

	_ = s.repo.UpdateStatus(ctx, backupID, model.BackupStatusRestoring, "", backup.FileSize, backup.DurationMs)

	for _, db := range backup.Databases {
		log.Info().Str("database", db).Msg("restoring database")
		if err := execRestore(db, backup.FilePath, s.containers); err != nil {
			_ = s.repo.UpdateStatus(ctx, backupID, model.BackupStatusCompleted, "", backup.FileSize, backup.DurationMs)
			return fmt.Errorf("restore %s failed: %w", db, err)
		}
		log.Info().Str("database", db).Msg("database restore completed")
	}

	_ = s.repo.UpdateStatus(ctx, backupID, model.BackupStatusCompleted, "", backup.FileSize, backup.DurationMs)

	log.Info().Str("id", backupID.String()).Msg("restore finished")
	return nil
}

func (s *BackupService) Delete(ctx context.Context, backupID uuid.UUID) error {
	backup, err := s.repo.FindByID(ctx, backupID)
	if err != nil {
		return fmt.Errorf("find backup: %w", err)
	}
	if backup == nil {
		return fmt.Errorf("backup not found")
	}

	if backup.FilePath != "" {
		os.RemoveAll(backup.FilePath)
	}

	return s.repo.Delete(ctx, backupID)
}

func (s *BackupService) Cleanup(ctx context.Context, retentionDays int) error {
	log := logger.With("backup")
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	backups, _, err := s.repo.List(ctx, 1000, 0)
	if err != nil {
		return err
	}

	for _, b := range backups {
		if b.StartedAt.Before(cutoff) {
			if b.FilePath != "" {
				os.RemoveAll(b.FilePath)
			}
		}
	}

	deleted, err := s.repo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		return err
	}

	if deleted > 0 {
		log.Info().Int64("deleted", deleted).Int("retention_days", retentionDays).Msg("old backups cleaned up")
	}
	return nil
}

func (s *BackupService) GetSettings(ctx context.Context) model.BackupSettings {
	settings, _ := s.settingsRepo.GetAll(ctx, "backup_")
	return ParseBackupSettings(settings)
}

func (s *BackupService) SaveSettings(ctx context.Context, bs model.BackupSettings) error {
	enabled := "false"
	if bs.Enabled {
		enabled = "true"
	}
	if err := s.settingsRepo.Set(ctx, "backup_enabled", enabled, false); err != nil {
		return err
	}
	if err := s.settingsRepo.Set(ctx, "backup_schedule", bs.Schedule, false); err != nil {
		return err
	}
	if err := s.settingsRepo.Set(ctx, "backup_retention_days", strconv.Itoa(bs.RetentionDays), false); err != nil {
		return err
	}
	if err := s.settingsRepo.Set(ctx, "backup_databases", strings.Join(bs.Databases, ","), false); err != nil {
		return err
	}
	return nil
}

func (s *BackupService) List(ctx context.Context, limit, offset int) ([]model.Backup, int, error) {
	return s.repo.List(ctx, limit, offset)
}

func (s *BackupService) FindByID(ctx context.Context, id uuid.UUID) (*model.Backup, error) {
	return s.repo.FindByID(ctx, id)
}

// ParseBackupSettings converts a settings map to BackupSettings.
func ParseBackupSettings(m map[string]string) model.BackupSettings {
	s := model.BackupSettings{
		Enabled:       m["enabled"] == "true",
		Schedule:      m["schedule"],
		RetentionDays: 30,
	}
	if s.Schedule == "" {
		s.Schedule = "daily"
	}
	if v, err := strconv.Atoi(m["retention_days"]); err == nil && v > 0 {
		s.RetentionDays = v
	}
	if dbs := m["databases"]; dbs != "" {
		s.Databases = strings.Split(dbs, ",")
	}
	return s
}

// shouldRunNow checks if the schedule matches the current time (within the 03:00 minute).
func shouldRunNow(schedule string, now time.Time, loc *time.Location) bool {
	t := now.In(loc)
	if t.Hour() != 3 || t.Minute() != 0 {
		return false
	}
	switch schedule {
	case "daily":
		return true
	case "weekly":
		return t.Weekday() == time.Sunday
	case "monthly":
		return t.Day() == 1
	}
	return false
}

func buildBackupDirName(t time.Time, trigger model.BackupTrigger) string {
	return fmt.Sprintf("%s_%s", t.Format("2006-01-02_150405"), string(trigger))
}

// execDump safely executes a database backup without shell interpolation (CWE-78).
func execDump(db string, backupPath string, cfg ContainerConfig) error {
	switch db {
	case "pg-platform":
		return pipeToGzip(
			exec.Command("docker", "exec", cfg.PGPlatformContainer, "pg_dumpall", "-U", cfg.PGPlatformUser),
			filepath.Join(backupPath, "pg-platform.sql.gz"),
		)
	case "pg-shared":
		return pipeToGzip(
			exec.Command("docker", "exec", cfg.PGSharedContainer, "pg_dumpall", "-U", cfg.PGSharedUser),
			filepath.Join(backupPath, "pg-shared.sql.gz"),
		)
	case "mongo-shared":
		outFile := filepath.Join(backupPath, "mongo-shared.archive.gz")
		cmd := exec.Command("docker", "exec", cfg.MongoContainer,
			"mongodump", "--authenticationDatabase", "admin",
			"-u", cfg.MongoUser, "-p", cfg.MongoPassword,
			"--archive", "--gzip")
		f, err := os.Create(outFile)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer f.Close()
		cmd.Stdout = f
		return cmd.Run()
	case "redis-shared":
		// BGSAVE
		bgsave := exec.Command("docker", "exec", cfg.RedisContainer,
			"redis-cli", "-a", cfg.RedisPassword, "BGSAVE")
		if out, err := bgsave.CombinedOutput(); err != nil {
			return fmt.Errorf("redis BGSAVE: %w — %s", err, string(out))
		}
		time.Sleep(2 * time.Second)
		// Copy RDB file out
		cp := exec.Command("docker", "cp",
			cfg.RedisContainer+":/data/dump.rdb",
			filepath.Join(backupPath, "redis-shared.rdb"))
		if out, err := cp.CombinedOutput(); err != nil {
			return fmt.Errorf("redis copy rdb: %w — %s", err, string(out))
		}
		return nil
	}
	return fmt.Errorf("unknown database: %s", db)
}

// execRestore safely restores a database from backup without shell interpolation (CWE-78).
func execRestore(db string, backupPath string, cfg ContainerConfig) error {
	switch db {
	case "pg-platform":
		return gunzipPipeTo(
			filepath.Join(backupPath, "pg-platform.sql.gz"),
			exec.Command("docker", "exec", "-i", cfg.PGPlatformContainer, "psql", "-U", cfg.PGPlatformUser),
		)
	case "pg-shared":
		return gunzipPipeTo(
			filepath.Join(backupPath, "pg-shared.sql.gz"),
			exec.Command("docker", "exec", "-i", cfg.PGSharedContainer, "psql", "-U", cfg.PGSharedUser),
		)
	case "mongo-shared":
		inFile := filepath.Join(backupPath, "mongo-shared.archive.gz")
		f, err := os.Open(inFile)
		if err != nil {
			return fmt.Errorf("open archive: %w", err)
		}
		defer f.Close()
		cmd := exec.Command("docker", "exec", "-i", cfg.MongoContainer,
			"mongorestore", "--authenticationDatabase", "admin",
			"-u", cfg.MongoUser, "-p", cfg.MongoPassword,
			"--archive", "--gzip", "--drop")
		cmd.Stdin = f
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("mongorestore: %w — %s", err, string(out))
		}
		return nil
	case "redis-shared":
		rdbFile := filepath.Join(backupPath, "redis-shared.rdb")
		cp := exec.Command("docker", "cp", rdbFile, cfg.RedisContainer+":/data/dump.rdb")
		if out, err := cp.CombinedOutput(); err != nil {
			return fmt.Errorf("redis copy rdb: %w — %s", err, string(out))
		}
		restart := exec.Command("docker", "restart", cfg.RedisContainer)
		if out, err := restart.CombinedOutput(); err != nil {
			return fmt.Errorf("redis restart: %w — %s", err, string(out))
		}
		return nil
	}
	return fmt.Errorf("unknown database: %s", db)
}

// pipeToGzip runs a command and pipes its stdout through gzip to a file.
func pipeToGzip(cmd *exec.Cmd, outPath string) error {
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer f.Close()

	gzipCmd := exec.Command("gzip")
	gzipCmd.Stdout = f

	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("create pipe: %w", err)
	}
	cmd.Stdout = pw
	gzipCmd.Stdin = pr

	if err := gzipCmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return fmt.Errorf("start gzip: %w", err)
	}

	if err := cmd.Run(); err != nil {
		pw.Close()
		pr.Close()
		return fmt.Errorf("run dump: %w", err)
	}
	pw.Close()

	if err := gzipCmd.Wait(); err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	return nil
}

// gunzipPipeTo decompresses a gzip file and pipes it to a command's stdin.
func gunzipPipeTo(inPath string, cmd *exec.Cmd) error {
	f, err := os.Open(inPath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	gunzipCmd := exec.Command("gunzip", "-c")
	gunzipCmd.Stdin = f

	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("create pipe: %w", err)
	}
	gunzipCmd.Stdout = pw
	cmd.Stdin = pr

	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return fmt.Errorf("start restore: %w", err)
	}

	if err := gunzipCmd.Run(); err != nil {
		pw.Close()
		pr.Close()
		return fmt.Errorf("gunzip: %w", err)
	}
	pw.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("restore: %w", err)
	}
	return nil
}

func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		size += info.Size()
		return nil
	})
	return size
}
