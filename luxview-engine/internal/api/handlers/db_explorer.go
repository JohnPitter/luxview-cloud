package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

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
// For postgres services, if the app has a custom DATABASE_URL env var, it overrides
// the provisioned URL so the explorer connects to the actual database the app uses.
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

	// For postgres: check if the app has a custom DATABASE_URL that differs from provisioned
	if svc.ServiceType == model.ServicePostgres && app.EnvVars != nil {
		var appEncrypted string
		if err := json.Unmarshal(app.EnvVars, &appEncrypted); err == nil {
			if appDecrypted, err := crypto.Decrypt(appEncrypted, h.encryptionKey); err == nil {
				var appEnv map[string]string
				if err := json.Unmarshal([]byte(appDecrypted), &appEnv); err == nil {
					if dbURL, ok := appEnv["DATABASE_URL"]; ok && dbURL != "" {
						creds["url"] = dbURL
					}
				}
			}
		}
	}

	return svc, creds, nil
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

// ListFiles lists objects in an S3 bucket.
func (h *ExplorerHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	svc, creds, err := h.decryptServiceCreds(r)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if svc.ServiceType != model.ServiceS3 {
		writeError(w, http.StatusBadRequest, "not an S3 service")
		return
	}

	prefix := r.URL.Query().Get("prefix")

	client, err := minio.New(creds["endpoint"], &minio.Options{
		Creds:  credentials.NewStaticV4(creds["access_key"], creds["secret_key"], ""),
		Secure: false,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to connect to storage")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	type fileInfo struct {
		Key          string    `json:"key"`
		Size         int64     `json:"size"`
		LastModified time.Time `json:"lastModified"`
		IsDir        bool      `json:"isDir"`
	}

	var files []fileInfo
	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: false,
	}

	for obj := range client.ListObjects(ctx, creds["bucket"], opts) {
		if obj.Err != nil {
			continue
		}
		isDir := strings.HasSuffix(obj.Key, "/")
		files = append(files, fileInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
			IsDir:        isDir,
		})
	}
	if files == nil {
		files = []fileInfo{}
	}

	writeJSON(w, http.StatusOK, files)
}

// UploadFile uploads a file to the S3 bucket.
func (h *ExplorerHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	log := logger.With("s3-explorer")
	svc, creds, err := h.decryptServiceCreds(r)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if svc.ServiceType != model.ServiceS3 {
		writeError(w, http.StatusBadRequest, "not an S3 service")
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

	client, err := minio.New(creds["endpoint"], &minio.Options{
		Creds:  credentials.NewStaticV4(creds["access_key"], creds["secret_key"], ""),
		Secure: false,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to connect to storage")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	_, err = client.PutObject(ctx, creds["bucket"], key, file, header.Size, minio.PutObjectOptions{
		ContentType: header.Header.Get("Content-Type"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to upload file")
		return
	}

	log.Info().Str("key", key).Int64("size", header.Size).Msg("file uploaded")
	writeJSON(w, http.StatusOK, map[string]string{"key": key, "message": "uploaded"})
}

// DownloadFile downloads a file from the S3 bucket.
func (h *ExplorerHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	svc, creds, err := h.decryptServiceCreds(r)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if svc.ServiceType != model.ServiceS3 {
		writeError(w, http.StatusBadRequest, "not an S3 service")
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}

	client, err := minio.New(creds["endpoint"], &minio.Options{
		Creds:  credentials.NewStaticV4(creds["access_key"], creds["secret_key"], ""),
		Secure: false,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to connect to storage")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	obj, err := client.GetObject(ctx, creds["bucket"], key, minio.GetObjectOptions{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get file")
		return
	}
	defer obj.Close()

	stat, err := obj.Stat()
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	// Extract filename from key
	parts := strings.Split(key, "/")
	filename := parts[len(parts)-1]

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Type", stat.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size, 10))
	io.Copy(w, obj)
}

// DeleteFile deletes a file from the S3 bucket.
func (h *ExplorerHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	log := logger.With("s3-explorer")
	svc, creds, err := h.decryptServiceCreds(r)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if svc.ServiceType != model.ServiceS3 {
		writeError(w, http.StatusBadRequest, "not an S3 service")
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}

	client, err := minio.New(creds["endpoint"], &minio.Options{
		Creds:  credentials.NewStaticV4(creds["access_key"], creds["secret_key"], ""),
		Secure: false,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to connect to storage")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if err := client.RemoveObject(ctx, creds["bucket"], key, minio.RemoveObjectOptions{}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete file")
		return
	}

	log.Info().Str("key", key).Msg("file deleted")
	writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
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
