package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
		// Each app gets its own Redis DB number (0-15) for isolation
		redisCount, _ := p.serviceRepo.CountByType(ctx, model.ServiceRedis)
		dbNum := redisCount % 16 // Redis supports DB 0-15 by default
		dbName = fmt.Sprintf("redis_%s", sanitizedID)
		// Each app gets its own password-prefixed key namespace via DB number
		creds = map[string]string{
			"host":     p.cfg.SharedRedisHost,
			"port":     fmt.Sprintf("%d", p.cfg.SharedRedisPort),
			"password": p.cfg.SharedRedisPassword,
			"db":       fmt.Sprintf("%d", dbNum),
			"url":      fmt.Sprintf("redis://:%s@%s:%d/%d", p.cfg.SharedRedisPassword, p.cfg.SharedRedisHost, p.cfg.SharedRedisPort, dbNum),
		}
		log.Info().Str("db", dbName).Int("redis_db", dbNum).Msg("redis provisioned")

	case model.ServiceMongoDB:
		dbName = fmt.Sprintf("app_%s", sanitizedID)
		userName := fmt.Sprintf("app_%s_user", sanitizedID)
		if err := p.provisionMongoDB(ctx, dbName, userName, password); err != nil {
			return nil, fmt.Errorf("provision mongodb: %w", err)
		}
		creds = map[string]string{
			"host":     p.cfg.SharedMongoHost,
			"port":     fmt.Sprintf("%d", p.cfg.SharedMongoPort),
			"database": dbName,
			"username": userName,
			"password": password,
			"url":      fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?authSource=%s", userName, password, p.cfg.SharedMongoHost, p.cfg.SharedMongoPort, dbName, dbName),
		}
		log.Info().Str("db", dbName).Str("user", userName).Msg("mongodb provisioned")

	case model.ServiceRabbitMQ:
		vhost := fmt.Sprintf("app_%s", sanitizedID)
		dbName = vhost
		userName := fmt.Sprintf("app_%s_user", sanitizedID)
		if err := p.provisionRabbitMQ(ctx, vhost, userName, password); err != nil {
			return nil, fmt.Errorf("provision rabbitmq: %w", err)
		}
		creds = map[string]string{
			"host":     p.cfg.SharedRabbitHost,
			"port":     fmt.Sprintf("%d", p.cfg.SharedRabbitPort),
			"vhost":    vhost,
			"username": userName,
			"password": password,
			"url":      fmt.Sprintf("amqp://%s:%s@%s:%d/%s", userName, password, p.cfg.SharedRabbitHost, p.cfg.SharedRabbitPort, vhost),
		}
		log.Info().Str("vhost", vhost).Str("user", userName).Msg("rabbitmq provisioned")

	case model.ServiceStorage:
		storageName := fmt.Sprintf("app-%s", strings.ReplaceAll(appID.String(), "_", "-"))
		dbName = storageName
		hostPath := filepath.Join(p.cfg.StorageBasePath, storageName)
		if err := p.provisionStorage(hostPath); err != nil {
			return nil, fmt.Errorf("provision storage: %w", err)
		}
		creds = map[string]string{
			"host_path":      hostPath,
			"container_path": "/storage",
		}
		log.Info().Str("path", hostPath).Msg("storage provisioned")

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
	log := logger.With("provisioner")

	// Use dedicated timeout so provisioning isn't cancelled by HTTP request timeout
	provCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	adminConnStr := fmt.Sprintf("postgres://%s:%s@%s:%d/postgres?sslmode=disable",
		p.cfg.SharedPGUser, p.cfg.SharedPGPassword, p.cfg.SharedPGHost, p.cfg.SharedPGPort)

	conn, err := pgx.Connect(provCtx, adminConnStr)
	if err != nil {
		return fmt.Errorf("connect to shared pg: %w", err)
	}
	defer conn.Close(provCtx)

	// Create user first (needed before OWNER assignment)
	log.Debug().Str("user", userName).Msg("creating postgres user")
	_, err = conn.Exec(provCtx, fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", quoteIdent(userName), password))
	if err != nil && !isPgError(err, "42710") { // 42710 = duplicate_object
		return fmt.Errorf("create user: %w", err)
	}

	// Create database owned by the app user — ensures full isolation
	log.Debug().Str("db", dbName).Msg("creating postgres database")
	_, err = conn.Exec(provCtx, fmt.Sprintf("CREATE DATABASE %s OWNER %s", quoteIdent(dbName), quoteIdent(userName)))
	if err != nil && !isPgError(err, "42P04") { // 42P04 = duplicate_database
		return fmt.Errorf("create database: %w", err)
	}

	log.Debug().Str("db", dbName).Msg("granting database privileges")
	_, err = conn.Exec(provCtx, fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", quoteIdent(dbName), quoteIdent(userName)))
	if err != nil {
		return fmt.Errorf("grant privileges: %w", err)
	}

	// Connect to the new database to grant schema permissions (PG 15+ requirement)
	dbConnStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		p.cfg.SharedPGUser, p.cfg.SharedPGPassword, p.cfg.SharedPGHost, p.cfg.SharedPGPort, dbName)
	dbConn, err := pgx.Connect(provCtx, dbConnStr)
	if err != nil {
		return fmt.Errorf("connect to new db: %w", err)
	}
	defer dbConn.Close(provCtx)

	// Grant schema permissions and set ownership so the app user can create tables
	log.Debug().Str("db", dbName).Msg("configuring schema permissions")
	if _, err := dbConn.Exec(provCtx, fmt.Sprintf("ALTER SCHEMA public OWNER TO %s", quoteIdent(userName))); err != nil {
		log.Warn().Err(err).Str("db", dbName).Msg("failed to alter schema owner")
	}
	if _, err := dbConn.Exec(provCtx, fmt.Sprintf("GRANT ALL ON SCHEMA public TO %s", quoteIdent(userName))); err != nil {
		log.Warn().Err(err).Str("db", dbName).Msg("failed to grant schema privileges")
	}
	// Revoke public access so other app users cannot access this database
	if _, err := dbConn.Exec(provCtx, "REVOKE ALL ON SCHEMA public FROM PUBLIC"); err != nil {
		log.Warn().Err(err).Str("db", dbName).Msg("failed to revoke public schema access")
	}
	if _, err := dbConn.Exec(provCtx, fmt.Sprintf("GRANT ALL ON SCHEMA public TO %s", quoteIdent(userName))); err != nil {
		log.Warn().Err(err).Str("db", dbName).Msg("failed to re-grant schema privileges")
	}

	return nil
}

// Deprovision removes a service and its resources.
func (p *Provisioner) Deprovision(ctx context.Context, svc *model.AppService) error {
	log := logger.With("provisioner")

	switch svc.ServiceType {
	case model.ServicePostgres:
		if err := p.deprovisionPostgres(ctx, svc.DBName); err != nil {
			log.Warn().Err(err).Str("db", svc.DBName).Msg("failed to deprovision postgres")
		}
	case model.ServiceMongoDB:
		if err := p.deprovisionMongoDB(ctx, svc.DBName); err != nil {
			log.Warn().Err(err).Str("db", svc.DBName).Msg("failed to deprovision mongodb")
		}
	case model.ServiceRabbitMQ:
		if err := p.deprovisionRabbitMQ(ctx, svc.DBName); err != nil {
			log.Warn().Err(err).Str("vhost", svc.DBName).Msg("failed to deprovision rabbitmq")
		}
	case model.ServiceStorage:
		if err := p.deprovisionStorage(svc.DBName); err != nil {
			log.Warn().Err(err).Str("storage", svc.DBName).Msg("failed to deprovision storage")
		}
	}

	if err := p.serviceRepo.Delete(ctx, svc.ID); err != nil {
		return err
	}

	log.Info().Str("service", string(svc.ServiceType)).Str("db", svc.DBName).Msg("service deprovisioned")
	return nil
}

// provisionMongoDB creates a dedicated user with readWrite access to a specific database.
// Uses docker exec + mongosh since the Go mongo driver would add a heavy dependency.
func (p *Provisioner) provisionMongoDB(ctx context.Context, dbName, userName, password string) error {
	log := logger.With("provisioner")

	// JavaScript command to create user scoped to the app database
	// dropUser first to handle re-provisioning gracefully
	jsCmd := fmt.Sprintf(`
		db = db.getSiblingDB('%s');
		try { db.dropUser('%s'); } catch(e) {}
		db.createUser({
			user: '%s',
			pwd: '%s',
			roles: [{ role: 'readWrite', db: '%s' }]
		});
	`, dbName, userName, userName, password, dbName)

	execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "docker", "exec", "luxview-mongo-shared",
		"mongosh",
		"--username", p.cfg.SharedMongoUser,
		"--password", p.cfg.SharedMongoPassword,
		"--authenticationDatabase", "admin",
		"--quiet",
		"--eval", jsCmd,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Warn().Err(err).Str("output", string(output)).Msg("mongosh user creation failed")
		return fmt.Errorf("mongosh exec failed: %s — %w", string(output), err)
	}

	log.Info().Str("db", dbName).Str("user", userName).Msg("mongodb user created")
	return nil
}

// provisionRabbitMQ creates a dedicated vhost and user with access only to that vhost.
// Uses the RabbitMQ Management HTTP API (port 15672).
func (p *Provisioner) provisionRabbitMQ(ctx context.Context, vhost, userName, password string) error {
	log := logger.With("provisioner")
	baseURL := fmt.Sprintf("http://%s:15672/api", p.cfg.SharedRabbitHost)

	client := &http.Client{Timeout: 15 * time.Second}

	// 1. Create vhost
	if err := p.rabbitAPIPut(ctx, client, baseURL, fmt.Sprintf("/vhosts/%s", vhost), "{}"); err != nil {
		return fmt.Errorf("create vhost: %w", err)
	}

	// 2. Create user with password
	userBody := fmt.Sprintf(`{"password":"%s","tags":""}`, password)
	if err := p.rabbitAPIPut(ctx, client, baseURL, fmt.Sprintf("/users/%s", userName), userBody); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	// 3. Grant permissions: user can configure, write, read everything in their vhost only
	permBody := `{"configure":".*","write":".*","read":".*"}`
	if err := p.rabbitAPIPut(ctx, client, baseURL, fmt.Sprintf("/permissions/%s/%s", vhost, userName), permBody); err != nil {
		return fmt.Errorf("set permissions: %w", err)
	}

	log.Info().Str("vhost", vhost).Str("user", userName).Msg("rabbitmq vhost+user created")
	return nil
}

// rabbitAPIPut sends a PUT request to the RabbitMQ Management API.
func (p *Provisioner) rabbitAPIPut(ctx context.Context, client *http.Client, baseURL, path, body string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, baseURL+path, bytes.NewBufferString(body))
	if err != nil {
		return err
	}
	req.SetBasicAuth(p.cfg.SharedRabbitUser, p.cfg.SharedRabbitPassword)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("rabbitmq API %s returned %d", path, resp.StatusCode)
	}
	return nil
}

func (p *Provisioner) deprovisionMongoDB(ctx context.Context, dbName string) error {
	userName := dbName + "_user"
	jsCmd := fmt.Sprintf(`
		db = db.getSiblingDB('%s');
		try { db.dropUser('%s'); } catch(e) {}
		db.dropDatabase();
	`, dbName, userName)

	ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx2, "docker", "exec", "luxview-mongo-shared",
		"mongosh",
		"--username", p.cfg.SharedMongoUser,
		"--password", p.cfg.SharedMongoPassword,
		"--authenticationDatabase", "admin",
		"--quiet",
		"--eval", jsCmd,
	)
	_, _ = cmd.CombinedOutput()
	return nil
}

func (p *Provisioner) deprovisionRabbitMQ(ctx context.Context, vhost string) error {
	baseURL := fmt.Sprintf("http://%s:15672/api", p.cfg.SharedRabbitHost)
	client := &http.Client{Timeout: 15 * time.Second}
	userName := vhost + "_user"

	// Delete user first, then vhost
	_ = p.rabbitAPIDelete(ctx, client, baseURL, fmt.Sprintf("/users/%s", userName))
	_ = p.rabbitAPIDelete(ctx, client, baseURL, fmt.Sprintf("/vhosts/%s", vhost))
	return nil
}

func (p *Provisioner) rabbitAPIDelete(ctx context.Context, client *http.Client, baseURL, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, baseURL+path, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(p.cfg.SharedRabbitUser, p.cfg.SharedRabbitPassword)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (p *Provisioner) provisionStorage(hostPath string) error {
	return os.MkdirAll(hostPath, 0755)
}

func (p *Provisioner) deprovisionStorage(storageName string) error {
	hostPath := filepath.Join(p.cfg.StorageBasePath, storageName)
	// Validate path is under storage base to prevent path traversal
	absPath, err := filepath.Abs(hostPath)
	if err != nil {
		return err
	}
	absBase, err := filepath.Abs(p.cfg.StorageBasePath)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) {
		return fmt.Errorf("invalid storage path: outside base directory")
	}
	return os.RemoveAll(absPath)
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
// Injects multiple common env var names so the app can use whichever it expects.
func (p *Provisioner) GetEnvVarsForService(svc *model.AppService, creds map[string]string) map[string]string {
	envVars := make(map[string]string)
	switch svc.ServiceType {
	case model.ServicePostgres:
		envVars["DATABASE_URL"] = creds["url"]
		// Spring Boot / JDBC compatible
		jdbcURL := fmt.Sprintf("jdbc:postgresql://%s:%s/%s", creds["host"], creds["port"], creds["database"])
		envVars["SPRING_DATASOURCE_URL"] = jdbcURL
		envVars["SPRING_DATASOURCE_USERNAME"] = creds["username"]
		envVars["SPRING_DATASOURCE_PASSWORD"] = creds["password"]
		envVars["PGHOST"] = creds["host"]
		envVars["PGPORT"] = creds["port"]
		envVars["PGDATABASE"] = creds["database"]
		envVars["PGUSER"] = creds["username"]
		envVars["PGPASSWORD"] = creds["password"]
	case model.ServiceRedis:
		envVars["REDIS_URL"] = creds["url"]
		envVars["REDIS_HOST"] = creds["host"]
		envVars["REDIS_PORT"] = creds["port"]
		envVars["REDIS_PASSWORD"] = creds["password"]
	case model.ServiceMongoDB:
		envVars["MONGODB_URL"] = creds["url"]
		envVars["MONGO_URL"] = creds["url"]
	case model.ServiceRabbitMQ:
		envVars["RABBITMQ_URL"] = creds["url"]
		envVars["AMQP_URL"] = creds["url"]
	case model.ServiceStorage:
		envVars["STORAGE_PATH"] = creds["container_path"]
	}
	return envVars
}

// GetStorageBinds returns Docker bind mount strings for storage services.
// Format: "host_path:container_path"
func (p *Provisioner) GetStorageBinds(svc *model.AppService, creds map[string]string) []string {
	if svc.ServiceType != model.ServiceStorage {
		return nil
	}
	hostPath := creds["host_path"]
	containerPath := creds["container_path"]
	if hostPath == "" || containerPath == "" {
		return nil
	}
	return []string{fmt.Sprintf("%s:%s", hostPath, containerPath)}
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
	// length/2 bytes → hex encodes to exactly length characters
	b := make([]byte, (length+1)/2)
	if _, err := rand.Read(b); err != nil {
		// Fallback: should never happen with crypto/rand
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)[:length]
}

// isPgError checks if the error is a PostgreSQL error with the given SQLSTATE code.
// Common codes: 42710=duplicate_object, 42P04=duplicate_database
func isPgError(err error, code string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == code
	}
	// Fallback to string matching for wrapped errors
	return err != nil && strings.Contains(err.Error(), "already exists")
}
