package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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

	case model.ServiceS3:
		bucketName := fmt.Sprintf("app-%s", strings.ReplaceAll(appID.String(), "_", "-"))
		dbName = bucketName
		if err := p.provisionS3(ctx, bucketName); err != nil {
			return nil, fmt.Errorf("provision s3: %w", err)
		}
		endpoint := fmt.Sprintf("%s:%d", p.cfg.SharedMinioHost, p.cfg.SharedMinioPort)
		creds = map[string]string{
			"endpoint":   endpoint,
			"bucket":     bucketName,
			"access_key": p.cfg.SharedMinioUser,
			"secret_key": p.cfg.SharedMinioPassword,
			"url":        fmt.Sprintf("http://%s/%s", endpoint, bucketName),
		}
		log.Info().Str("bucket", bucketName).Msg("s3 provisioned")

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
	adminConnStr := fmt.Sprintf("postgres://%s:%s@%s:%d/postgres?sslmode=disable",
		p.cfg.SharedPGUser, p.cfg.SharedPGPassword, p.cfg.SharedPGHost, p.cfg.SharedPGPort)

	conn, err := pgx.Connect(ctx, adminConnStr)
	if err != nil {
		return fmt.Errorf("connect to shared pg: %w", err)
	}
	defer conn.Close(ctx)

	// Create user first (needed before OWNER assignment)
	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", quoteIdent(userName), password))
	if err != nil && !isDuplicateRole(err) {
		return fmt.Errorf("create user: %w", err)
	}

	// Create database owned by the app user — ensures full isolation
	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s OWNER %s", quoteIdent(dbName), quoteIdent(userName)))
	if err != nil && !isDuplicateDB(err) {
		return fmt.Errorf("create database: %w", err)
	}

	_, err = conn.Exec(ctx, fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", quoteIdent(dbName), quoteIdent(userName)))
	if err != nil {
		return fmt.Errorf("grant privileges: %w", err)
	}

	// Connect to the new database to grant schema permissions (PG 15+ requirement)
	dbConnStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		p.cfg.SharedPGUser, p.cfg.SharedPGPassword, p.cfg.SharedPGHost, p.cfg.SharedPGPort, dbName)
	dbConn, err := pgx.Connect(ctx, dbConnStr)
	if err != nil {
		return fmt.Errorf("connect to new db: %w", err)
	}
	defer dbConn.Close(ctx)

	// Grant schema permissions and set ownership so the app user can create tables
	_, _ = dbConn.Exec(ctx, fmt.Sprintf("ALTER SCHEMA public OWNER TO %s", quoteIdent(userName)))
	_, _ = dbConn.Exec(ctx, fmt.Sprintf("GRANT ALL ON SCHEMA public TO %s", quoteIdent(userName)))
	// Revoke public access so other app users cannot access this database
	_, _ = dbConn.Exec(ctx, "REVOKE ALL ON SCHEMA public FROM PUBLIC")
	_, _ = dbConn.Exec(ctx, fmt.Sprintf("GRANT ALL ON SCHEMA public TO %s", quoteIdent(userName)))

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
	case model.ServiceS3:
		if err := p.deprovisionS3(ctx, svc.DBName); err != nil {
			log.Warn().Err(err).Str("bucket", svc.DBName).Msg("failed to deprovision s3")
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

func (p *Provisioner) provisionS3(ctx context.Context, bucketName string) error {
	endpoint := fmt.Sprintf("%s:%d", p.cfg.SharedMinioHost, p.cfg.SharedMinioPort)
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(p.cfg.SharedMinioUser, p.cfg.SharedMinioPassword, ""),
		Secure: false,
	})
	if err != nil {
		return fmt.Errorf("connect to minio: %w", err)
	}

	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}
	}
	return nil
}

func (p *Provisioner) deprovisionS3(ctx context.Context, bucketName string) error {
	endpoint := fmt.Sprintf("%s:%d", p.cfg.SharedMinioHost, p.cfg.SharedMinioPort)
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(p.cfg.SharedMinioUser, p.cfg.SharedMinioPassword, ""),
		Secure: false,
	})
	if err != nil {
		return err
	}

	// Remove all objects first
	objectsCh := client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{Recursive: true})
	for obj := range objectsCh {
		if obj.Err != nil {
			continue
		}
		_ = client.RemoveObject(ctx, bucketName, obj.Key, minio.RemoveObjectOptions{})
	}

	return client.RemoveBucket(ctx, bucketName)
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
	case model.ServiceS3:
		envVars["S3_ENDPOINT"] = fmt.Sprintf("http://%s", creds["endpoint"])
		envVars["S3_BUCKET"] = creds["bucket"]
		envVars["S3_ACCESS_KEY"] = creds["access_key"]
		envVars["S3_SECRET_KEY"] = creds["secret_key"]
		envVars["AWS_ENDPOINT_URL"] = fmt.Sprintf("http://%s", creds["endpoint"])
		envVars["AWS_ACCESS_KEY_ID"] = creds["access_key"]
		envVars["AWS_SECRET_ACCESS_KEY"] = creds["secret_key"]
		envVars["AWS_DEFAULT_REGION"] = "us-east-1"
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
