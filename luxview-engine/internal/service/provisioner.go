package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/luxview/engine/internal/config"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/crypto"
	"github.com/luxview/engine/pkg/logger"
)

// Provisioner creates and manages shared services (databases, caches).
type Provisioner struct {
	serviceRepo   *repository.ServiceRepo
	cfg           *config.Config
	encryptionKey []byte
}

func NewProvisioner(serviceRepo *repository.ServiceRepo, cfg *config.Config, encryptionKey []byte) *Provisioner {
	return &Provisioner{
		serviceRepo:   serviceRepo,
		cfg:           cfg,
		encryptionKey: encryptionKey,
	}
}

// Provision creates a new service instance for the given app.
func (p *Provisioner) Provision(ctx context.Context, appID uuid.UUID, serviceType model.ServiceType) (*model.AppService, error) {
	log := logger.With("provisioner")

	// Check if service already exists for this app
	existing, err := p.serviceRepo.FindByAppAndType(ctx, appID, serviceType)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("service %s already provisioned for this app", serviceType)
	}

	sanitizedID := sanitizeForDB(appID.String())
	password := generatePassword(24)

	var creds map[string]string
	var dbName string

	switch serviceType {
	case model.ServicePostgres:
		dbName = fmt.Sprintf("app_%s", sanitizedID)
		userName := fmt.Sprintf("app_%s_user", sanitizedID)
		if err := p.provisionPostgres(ctx, dbName, userName, password); err != nil {
			return nil, fmt.Errorf("provision postgres: %w", err)
		}
		creds = map[string]string{
			"host":     p.cfg.SharedPGHost,
			"port":     fmt.Sprintf("%d", p.cfg.SharedPGPort),
			"database": dbName,
			"username": userName,
			"password": password,
			"url":      fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", userName, password, p.cfg.SharedPGHost, p.cfg.SharedPGPort, dbName),
		}
		log.Info().Str("db", dbName).Msg("postgres provisioned")

	case model.ServiceRedis:
		dbNum := 0 // Could be improved with a counter
		dbName = fmt.Sprintf("redis_%s", sanitizedID)
		creds = map[string]string{
			"host":     p.cfg.SharedRedisHost,
			"port":     fmt.Sprintf("%d", p.cfg.SharedRedisPort),
			"password": p.cfg.SharedRedisPassword,
			"db":       fmt.Sprintf("%d", dbNum),
			"url":      fmt.Sprintf("redis://:%s@%s:%d/%d", p.cfg.SharedRedisPassword, p.cfg.SharedRedisHost, p.cfg.SharedRedisPort, dbNum),
		}
		log.Info().Str("db", dbName).Msg("redis provisioned")

	case model.ServiceMongoDB:
		dbName = fmt.Sprintf("app_%s", sanitizedID)
		creds = map[string]string{
			"host":     p.cfg.SharedMongoHost,
			"port":     fmt.Sprintf("%d", p.cfg.SharedMongoPort),
			"database": dbName,
			"username": p.cfg.SharedMongoUser,
			"password": p.cfg.SharedMongoPassword,
			"url":      fmt.Sprintf("mongodb://%s:%s@%s:%d/%s", p.cfg.SharedMongoUser, p.cfg.SharedMongoPassword, p.cfg.SharedMongoHost, p.cfg.SharedMongoPort, dbName),
		}
		log.Info().Str("db", dbName).Msg("mongodb provisioned")

	case model.ServiceRabbitMQ:
		vhost := fmt.Sprintf("app_%s", sanitizedID)
		dbName = vhost
		creds = map[string]string{
			"host":     p.cfg.SharedRabbitHost,
			"port":     fmt.Sprintf("%d", p.cfg.SharedRabbitPort),
			"vhost":    vhost,
			"username": p.cfg.SharedRabbitUser,
			"password": p.cfg.SharedRabbitPassword,
			"url":      fmt.Sprintf("amqp://%s:%s@%s:%d/%s", p.cfg.SharedRabbitUser, p.cfg.SharedRabbitPassword, p.cfg.SharedRabbitHost, p.cfg.SharedRabbitPort, vhost),
		}
		log.Info().Str("vhost", vhost).Msg("rabbitmq provisioned")

	default:
		return nil, fmt.Errorf("unsupported service type: %s", serviceType)
	}

	// Encrypt credentials
	credsJSON, _ := json.Marshal(creds)
	encrypted, err := crypto.Encrypt(string(credsJSON), p.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt credentials: %w", err)
	}

	svc := &model.AppService{
		AppID:       appID,
		ServiceType: serviceType,
		DBName:      dbName,
		Credentials: json.RawMessage(fmt.Sprintf("%q", encrypted)),
	}

	if err := p.serviceRepo.Create(ctx, svc); err != nil {
		return nil, err
	}

	svc.CredentialsPlain = creds
	return svc, nil
}

func (p *Provisioner) provisionPostgres(ctx context.Context, dbName, userName, password string) error {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/postgres?sslmode=disable",
		p.cfg.SharedPGUser, p.cfg.SharedPGPassword, p.cfg.SharedPGHost, p.cfg.SharedPGPort)

	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return fmt.Errorf("connect to shared pg: %w", err)
	}
	defer conn.Close(ctx)

	// Create database (cannot use parameterized queries for DDL)
	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", quoteIdent(dbName)))
	if err != nil && !isDuplicateDB(err) {
		return fmt.Errorf("create database: %w", err)
	}

	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", quoteIdent(userName), password))
	if err != nil && !isDuplicateRole(err) {
		return fmt.Errorf("create user: %w", err)
	}

	_, err = conn.Exec(ctx, fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", quoteIdent(dbName), quoteIdent(userName)))
	if err != nil {
		return fmt.Errorf("grant privileges: %w", err)
	}

	return nil
}

// Deprovision removes a service and its resources.
func (p *Provisioner) Deprovision(ctx context.Context, svc *model.AppService) error {
	log := logger.With("provisioner")

	if svc.ServiceType == model.ServicePostgres {
		if err := p.deprovisionPostgres(ctx, svc.DBName); err != nil {
			log.Warn().Err(err).Str("db", svc.DBName).Msg("failed to deprovision postgres")
		}
	}

	if err := p.serviceRepo.Delete(ctx, svc.ID); err != nil {
		return err
	}

	log.Info().Str("service", string(svc.ServiceType)).Str("db", svc.DBName).Msg("service deprovisioned")
	return nil
}

func (p *Provisioner) deprovisionPostgres(ctx context.Context, dbName string) error {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/postgres?sslmode=disable",
		p.cfg.SharedPGUser, p.cfg.SharedPGPassword, p.cfg.SharedPGHost, p.cfg.SharedPGPort)

	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	userName := dbName + "_user"
	_, _ = conn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", quoteIdent(dbName)))
	_, _ = conn.Exec(ctx, fmt.Sprintf("DROP USER IF EXISTS %s", quoteIdent(userName)))
	return nil
}

// GetEnvVarsForService returns the env vars a service injects into the app container.
func (p *Provisioner) GetEnvVarsForService(svc *model.AppService, creds map[string]string) map[string]string {
	envVars := make(map[string]string)
	switch svc.ServiceType {
	case model.ServicePostgres:
		envVars["DATABASE_URL"] = creds["url"]
	case model.ServiceRedis:
		envVars["REDIS_URL"] = creds["url"]
	case model.ServiceMongoDB:
		envVars["MONGODB_URL"] = creds["url"]
	case model.ServiceRabbitMQ:
		envVars["RABBITMQ_URL"] = creds["url"]
	}
	return envVars
}

func sanitizeForDB(s string) string {
	return strings.ReplaceAll(s, "-", "_")
}

func quoteIdent(s string) string {
	// Simple identifier quoting. Only allow alphanumeric and underscore.
	clean := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return -1
	}, s)
	return clean
}

func generatePassword(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:length]
}

func isDuplicateDB(err error) bool {
	return err != nil && strings.Contains(err.Error(), "already exists")
}

func isDuplicateRole(err error) bool {
	return err != nil && strings.Contains(err.Error(), "already exists")
}
