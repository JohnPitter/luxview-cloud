package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/crypto"
	"github.com/luxview/engine/pkg/logger"
)

type ExplorerHandler struct {
	serviceRepo   *repository.ServiceRepo
	appRepo       *repository.AppRepo
	encryptionKey []byte
}

func NewExplorerHandler(
	serviceRepo *repository.ServiceRepo,
	appRepo *repository.AppRepo,
	encryptionKey []byte,
) *ExplorerHandler {
	return &ExplorerHandler{
		serviceRepo:   serviceRepo,
		appRepo:       appRepo,
		encryptionKey: encryptionKey,
	}
}

// decryptServiceCreds validates ownership and returns decrypted credentials.
// Uses the provisioned credentials by default. For postgres, if the app has a
// custom DATABASE_URL env var, the explorer connects using the provisioned user
// but targets the database from the custom URL — ensuring the user only accesses
// databases they own while seeing the actual data their app uses.
func (h *ExplorerHandler) decryptServiceCreds(r *http.Request) (*model.AppService, map[string]string, error) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	svcID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid service ID")
	}

	svc, err := h.serviceRepo.FindByID(ctx, svcID)
	if err != nil || svc == nil {
		return nil, nil, fmt.Errorf("service not found")
	}

	app, err := h.appRepo.FindByID(ctx, svc.AppID)
	if err != nil || app == nil || app.UserID != userID {
		return nil, nil, fmt.Errorf("forbidden")
	}

	var encrypted string
	if err := json.Unmarshal(svc.Credentials, &encrypted); err != nil {
		return nil, nil, fmt.Errorf("invalid credentials")
	}
	decrypted, err := crypto.Decrypt(encrypted, h.encryptionKey)
	if err != nil {
		return nil, nil, fmt.Errorf("decrypt: %w", err)
	}
	var creds map[string]string
	if err := json.Unmarshal([]byte(decrypted), &creds); err != nil {
		return nil, nil, fmt.Errorf("parse credentials: %w", err)
	}

	// For postgres: if the app overrides DATABASE_URL, rebuild the connection URL
	// using the provisioned user/password but targeting the custom database name.
	// This preserves isolation (app user can only access granted databases).
	if svc.ServiceType == model.ServicePostgres && app.EnvVars != nil {
		var appEncrypted string
		if err := json.Unmarshal(app.EnvVars, &appEncrypted); err == nil {
			if appDecrypted, err := crypto.Decrypt(appEncrypted, h.encryptionKey); err == nil {
				var appEnv map[string]string
				if err := json.Unmarshal([]byte(appDecrypted), &appEnv); err == nil {
					if dbURL, ok := appEnv["DATABASE_URL"]; ok && dbURL != "" && dbURL != creds["url"] {
						// Extract database name from the custom URL
						if dbName := extractDBNameFromURL(dbURL); dbName != "" && dbName != creds["database"] {
							// Rebuild URL with provisioned auth but custom database
							creds["url"] = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
								creds["username"], creds["password"], creds["host"], creds["port"], dbName)
							creds["database"] = dbName
						}
					}
				}
			}
		}
	}

	return svc, creds, nil
}

// extractDBNameFromURL extracts the database name from a postgres connection URL.
func extractDBNameFromURL(url string) string {
	// Format: postgres://user:pass@host:port/dbname?params
	// Find the last / before any ?
	clean := url
	if idx := strings.Index(clean, "?"); idx != -1 {
		clean = clean[:idx]
	}
	if idx := strings.LastIndex(clean, "/"); idx != -1 {
		return clean[idx+1:]
	}
	return ""
}

// ListTables lists tables (postgres) or returns an error for unsupported types.
func (h *ExplorerHandler) ListTables(w http.ResponseWriter, r *http.Request) {
	svc, creds, err := h.decryptServiceCreds(r)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	switch svc.ServiceType {
	case model.ServicePostgres:
		conn, err := pgx.Connect(ctx, creds["url"])
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to connect to database")
			return
		}
		defer conn.Close(ctx)

		rows, err := conn.Query(ctx, `
			SELECT table_name, table_type
			FROM information_schema.tables
			WHERE table_schema = 'public'
			ORDER BY table_name
		`)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list tables")
			return
		}
		defer rows.Close()

		type tableInfo struct {
			Name string `json:"name"`
			Type string `json:"type"`
		}
		var tables []tableInfo
		for rows.Next() {
			var t tableInfo
			if err := rows.Scan(&t.Name, &t.Type); err == nil {
				tables = append(tables, t)
			}
		}
		if tables == nil {
			tables = []tableInfo{}
		}
		writeJSON(w, http.StatusOK, tables)

	default:
		writeError(w, http.StatusBadRequest, "table listing not supported for this service type")
	}
}

// GetTableSchema returns column info for a table.
func (h *ExplorerHandler) GetTableSchema(w http.ResponseWriter, r *http.Request) {
	svc, creds, err := h.decryptServiceCreds(r)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	tableName := chi.URLParam(r, "table")
	if tableName == "" {
		writeError(w, http.StatusBadRequest, "table name required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	switch svc.ServiceType {
	case model.ServicePostgres:
		conn, err := pgx.Connect(ctx, creds["url"])
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to connect")
			return
		}
		defer conn.Close(ctx)

		rows, err := conn.Query(ctx, `
			SELECT column_name, data_type, is_nullable, column_default
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1
			ORDER BY ordinal_position
		`, tableName)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get schema")
			return
		}
		defer rows.Close()

		type columnInfo struct {
			Name     string  `json:"name"`
			Type     string  `json:"type"`
			Nullable string  `json:"nullable"`
			Default  *string `json:"default"`
		}
		var columns []columnInfo
		for rows.Next() {
			var c columnInfo
			if err := rows.Scan(&c.Name, &c.Type, &c.Nullable, &c.Default); err == nil {
				columns = append(columns, c)
			}
		}
		if columns == nil {
			columns = []columnInfo{}
		}

		// Also get row count
		var count int64
		_ = conn.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdentPG(tableName))).Scan(&count)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"columns":  columns,
			"rowCount": count,
		})

	default:
		writeError(w, http.StatusBadRequest, "schema not supported for this service type")
	}
}

// ExecuteQuery runs an arbitrary SQL query against the user's database.
func (h *ExplorerHandler) ExecuteQuery(w http.ResponseWriter, r *http.Request) {
	log := logger.With("db-explorer")
	svc, creds, err := h.decryptServiceCreds(r)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Query) == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}

	// Limit query length
	if len(req.Query) > 10000 {
		writeError(w, http.StatusBadRequest, "query too long (max 10000 chars)")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	switch svc.ServiceType {
	case model.ServicePostgres:
		conn, err := pgx.Connect(ctx, creds["url"])
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to connect to database")
			return
		}
		defer conn.Close(ctx)

		rows, err := conn.Query(ctx, req.Query)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("query error: %s", err.Error()))
			return
		}
		defer rows.Close()

		fieldDescs := rows.FieldDescriptions()
		columns := make([]string, len(fieldDescs))
		for i, fd := range fieldDescs {
			columns[i] = string(fd.Name)
		}

		var resultRows []map[string]interface{}
		rowCount := 0
		maxRows := 1000

		for rows.Next() {
			if rowCount >= maxRows {
				break
			}
			values, err := rows.Values()
			if err != nil {
				continue
			}
			row := make(map[string]interface{})
			for i, col := range columns {
				row[col] = values[i]
			}
			resultRows = append(resultRows, row)
			rowCount++
		}

		if resultRows == nil {
			resultRows = []map[string]interface{}{}
		}

		log.Info().Int("rows", rowCount).Msg("query executed")
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"columns":  columns,
			"rows":     resultRows,
			"rowCount": rowCount,
			"truncated": rowCount >= maxRows,
		})

	default:
		writeError(w, http.StatusBadRequest, "queries not supported for this service type")
	}
}

// resolveStoragePath validates and resolves a storage path, preventing path traversal.
func resolveStoragePath(basePath, key string) (string, error) {
	fullPath := filepath.Join(basePath, filepath.FromSlash(key))
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return "", fmt.Errorf("path traversal detected")
	}
	return absPath, nil
}

// ListFiles lists files in the local storage directory.
func (h *ExplorerHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	svc, creds, err := h.decryptServiceCreds(r)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if svc.ServiceType != model.ServiceStorage {
		writeError(w, http.StatusBadRequest, "not a storage service")
		return
	}

	basePath := creds["host_path"]
	prefix := r.URL.Query().Get("prefix")

	dirPath, err := resolveStoragePath(basePath, prefix)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	type fileInfo struct {
		Key          string    `json:"key"`
		Size         int64     `json:"size"`
		LastModified time.Time `json:"lastModified"`
		IsDir        bool      `json:"isDir"`
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, []fileInfo{})
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list files")
		return
	}

	files := make([]fileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		key := prefix + entry.Name()
		if entry.IsDir() {
			key += "/"
		}
		files = append(files, fileInfo{
			Key:          key,
			Size:         info.Size(),
			LastModified: info.ModTime(),
			IsDir:        entry.IsDir(),
		})
	}

	writeJSON(w, http.StatusOK, files)
}

// UploadFile uploads a file to the local storage directory.
func (h *ExplorerHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	log := logger.With("storage-explorer")
	svc, creds, err := h.decryptServiceCreds(r)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if svc.ServiceType != model.ServiceStorage {
		writeError(w, http.StatusBadRequest, "not a storage service")
		return
	}

	// Max 50MB
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "file too large (max 50MB)")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	key := r.FormValue("key")
	if key == "" {
		key = header.Filename
	}

	basePath := creds["host_path"]

	// Enforce storage quota from user's plan
	user := middleware.GetUser(r.Context())
	if user != nil && user.Plan != nil && user.Plan.MaxDiskPerApp != "" {
		planLimit := parseMemoryString(user.Plan.MaxDiskPerApp)
		if planLimit > 0 {
			currentUsage, _ := calculateDirSize(basePath)
			if currentUsage+header.Size > planLimit {
				writeError(w, http.StatusForbidden, fmt.Sprintf(
					"storage quota exceeded: used %s + %s would exceed limit of %s",
					formatBytes(currentUsage), formatBytes(header.Size), user.Plan.MaxDiskPerApp,
				))
				return
			}
		}
	}

	destPath, err := resolveStoragePath(basePath, key)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create directory")
		return
	}

	dst, err := os.Create(destPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create file")
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write file")
		return
	}

	log.Info().Str("key", key).Int64("size", written).Msg("file uploaded")
	writeJSON(w, http.StatusOK, map[string]string{"key": key, "message": "uploaded"})
}

// DownloadFile downloads a file from the local storage directory.
func (h *ExplorerHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	svc, creds, err := h.decryptServiceCreds(r)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if svc.ServiceType != model.ServiceStorage {
		writeError(w, http.StatusBadRequest, "not a storage service")
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}

	basePath := creds["host_path"]
	filePath, err := resolveStoragePath(basePath, key)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	info, err := os.Stat(filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to open file")
		return
	}
	defer file.Close()

	// Extract filename from key
	parts := strings.Split(key, "/")
	filename := parts[len(parts)-1]

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	io.Copy(w, file)
}

// DeleteFile deletes a file from the local storage directory.
func (h *ExplorerHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	log := logger.With("storage-explorer")
	svc, creds, err := h.decryptServiceCreds(r)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if svc.ServiceType != model.ServiceStorage {
		writeError(w, http.StatusBadRequest, "not a storage service")
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}

	basePath := creds["host_path"]
	filePath, err := resolveStoragePath(basePath, key)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete file")
		return
	}

	log.Info().Str("key", key).Msg("file deleted")
	writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

// formatBytes formats bytes into a human-readable string.
func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1fGB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1fMB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1fKB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

// calculateDirSize walks a directory tree and returns the total size in bytes.
func calculateDirSize(path string) (int64, error) {
	var total int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// ServiceUsage returns the current disk/resource usage and plan limit for any service type.
func (h *ExplorerHandler) ServiceUsage(w http.ResponseWriter, r *http.Request) {
	svc, creds, err := h.decryptServiceCreds(r)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	var used int64

	switch svc.ServiceType {
	case model.ServiceStorage:
		basePath := creds["host_path"]
		used, _ = calculateDirSize(basePath)

	case model.ServicePostgres:
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		conn, connErr := pgx.Connect(ctx, creds["url"])
		if connErr == nil {
			defer conn.Close(ctx)
			_ = conn.QueryRow(ctx, "SELECT pg_database_size(current_database())").Scan(&used)
		}

	case model.ServiceRedis:
		// Redis is in-memory; report memory usage via INFO
		// For now, report 0 — redis usage is ephemeral
		used = 0

	case model.ServiceMongoDB:
		// MongoDB db.stats() requires mongosh; skip for now
		used = 0

	case model.ServiceRabbitMQ:
		// RabbitMQ doesn't expose per-vhost disk usage easily
		used = 0
	}

	user := middleware.GetUser(r.Context())
	var limitStr string
	if user != nil && user.Plan != nil {
		limitStr = user.Plan.MaxDiskPerApp
	}
	var limit int64
	if limitStr != "" {
		limit = parseMemoryString(limitStr)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"used":     used,
		"limit":    limit,
		"limitStr": limitStr,
	})
}

func quoteIdentPG(s string) string {
	clean := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return -1
	}, s)
	return `"` + clean + `"`
}
