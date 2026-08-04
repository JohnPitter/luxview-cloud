package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/luxview/engine/pkg/logger"
)

const globalStorageMaxUpload = int64(4 << 30)

// GlobalStorageHandler exposes the platform asset directory to administrators.
// App service directories remain outside this root and keep their isolation.
type GlobalStorageHandler struct {
	rootPath string
}

func NewGlobalStorageHandler(storageBasePath string) *GlobalStorageHandler {
	return &GlobalStorageHandler{rootPath: filepath.Join(storageBasePath, "_global")}
}

func (h *GlobalStorageHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	dirPath, err := resolveStoragePath(h.rootPath, prefix)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, []storageFileInfo{})
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list global storage")
		return
	}

	files := make([]storageFileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		key := prefix + entry.Name()
		if entry.IsDir() {
			key += "/"
		}
		files = append(files, storageFileInfo{Key: key, Size: info.Size(), LastModified: info.ModTime(), IsDir: entry.IsDir()})
	}
	writeJSON(w, http.StatusOK, files)
}

func (h *GlobalStorageHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	// Large client uploads can exceed the server's default 15-second read deadline.
	if rc := http.NewResponseController(w); rc != nil {
		_ = rc.SetReadDeadline(time.Now().Add(30 * time.Minute))
	}
	r.Body = http.MaxBytesReader(w, r.Body, globalStorageMaxUpload)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "arquivo muito grande ou formulário inválido")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "arquivo é obrigatório")
		return
	}
	defer file.Close()

	key := r.FormValue("key")
	if key == "" {
		key = header.Filename
	}
	destPath, err := resolveStoragePath(h.rootPath, key)
	if err != nil || filepath.Base(destPath) == "_global" {
		writeError(w, http.StatusBadRequest, "caminho inválido")
		return
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao criar diretório")
		return
	}

	tmp, err := os.CreateTemp(filepath.Dir(destPath), ".upload-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao criar arquivo temporário")
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	written, copyErr := io.Copy(tmp, io.LimitReader(file, globalStorageMaxUpload+1))
	closeErr := tmp.Close()
	if copyErr != nil || closeErr != nil {
		writeError(w, http.StatusInternalServerError, "falha ao gravar arquivo")
		return
	}
	if written > globalStorageMaxUpload {
		writeError(w, http.StatusRequestEntityTooLarge, "arquivo muito grande")
		return
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao finalizar upload")
		return
	}

	log := logger.With("global-storage")
	log.Info().Str("key", key).Int64("size", written).Msg("global file uploaded")
	writeJSON(w, http.StatusOK, map[string]string{"key": key, "message": "uploaded"})
}

func (h *GlobalStorageHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}
	filePath, err := resolveStoragePath(h.rootPath, key)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	file, err := os.Open(filePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to open file")
		return
	}
	defer file.Close()
	if rc := http.NewResponseController(w); rc != nil {
		_ = rc.SetWriteDeadline(time.Time{})
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(key)))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	if _, err := io.Copy(w, file); err != nil {
		log := logger.With("global-storage")
		log.Error().Err(err).Str("key", key).Msg("failed to stream global file")
	}
}

func (h *GlobalStorageHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}
	filePath, err := resolveStoragePath(h.rootPath, key)
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
	log := logger.With("global-storage")
	log.Info().Str("key", key).Msg("global file deleted")
	writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func (h *GlobalStorageHandler) Usage(w http.ResponseWriter, _ *http.Request) {
	used, _ := calculateDirSize(h.rootPath)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"used": used, "limit": 0, "limitStr": "sem limite", "updatedAt": time.Now(),
	})
}

type storageFileInfo struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"lastModified"`
	IsDir        bool      `json:"isDir"`
}
