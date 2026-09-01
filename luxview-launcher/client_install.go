package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const clientBaseHashFileName = "luxview-base.hash"

func usesSplitClient(card GameCard) bool {
	return strings.TrimSpace(card.BaseURL) != "" &&
		strings.TrimSpace(card.PatchURL) != "" &&
		strings.TrimSpace(card.BaseHash) != ""
}

// shouldDownloadClientBase is true on first install and when the published zip
// itself changed. A missing local stamp (legacy install) does not re-fetch the
// multi-hundred-MiB zip just to apply a config overlay.
func shouldDownloadClientBase(installed bool, localBase, catalogBase string) bool {
	if !installed {
		return true
	}
	if catalogBase == "" {
		return true
	}
	if localBase == "" {
		return false
	}
	return localBase != catalogBase
}

func installedBaseHash(appID string) string {
	dir, err := installDir(appID)
	if err != nil {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(dir, clientBaseHashFileName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func saveInstalledBaseHash(appID, hash string) error {
	hash = strings.TrimSpace(hash)
	if appID == "" || hash == "" {
		return nil
	}
	dir, err := installDir(appID)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, clientBaseHashFileName), []byte(hash+"\n"), 0o644)
}

func clientBaseCachePath(game, baseHash string) (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	game = sanitizeCacheKey(game)
	baseHash = sanitizeCacheKey(baseHash)
	if game == "" || baseHash == "" {
		return "", fmt.Errorf("cache de client inválido")
	}
	return filepath.Join(base, "LuxViewLauncher", "bases", game, baseHash+".zip"), nil
}

func sanitizeCacheKey(raw string) string {
	raw = strings.TrimSpace(raw)
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (a *App) installSplitClient(card GameCard, dir string, updating bool) error {
	needBase := shouldDownloadClientBase(updating, installedBaseHash(card.AppID), card.BaseHash)
	// O patch do Tibia contém só o init.lua (IP do servidor). Se o client_hash
	// mudou, o conteúdo real do client mudou (módulos/overlay) e instalações
	// legadas sem stamp de base precisam reextrair o zip base inteiro.
	if !needBase && updating && normalizeGameID(card.Game) == "tibia" {
		localClient := installedClientHash(card.AppID)
		if card.ClientHash != "" && localClient != card.ClientHash {
			needBase = true
		}
	}
	if !needBase {
		game := normalizeGameID(card.Game)
		if spec, ok := launchSpecForGame(game); ok && !clientFilesReady(dir, game, spec) {
			needBase = true
		}
	}
	if needBase {
		cachePath, err := a.ensureClientBaseCache(card)
		if err != nil {
			return err
		}
		a.progress(card.Game, "extract", 0)
		restoreSettings := a.backupGameSettings(card, updating)
		if err := unzip(cachePath, dir, func(done, count int) {
			if count > 0 {
				a.progress(card.Game, "extract", int(float64(done)/float64(count)*100))
			}
		}); err != nil {
			if restoreSettings != nil {
				restoreSettings()
			}
			return fmt.Errorf("falha ao extrair o client: %w", err)
		}
		if restoreSettings != nil {
			restoreSettings()
		}
	}

	a.progressMsg(card.Game, "download", -1, "atualização (só arquivos alterados)")
	if err := a.downloadAndExtractZip(card, card.PatchURL, dir, updating && !needBase); err != nil {
		return fmt.Errorf("falha no patch do client: %w", err)
	}
	return a.finishClientInstall(card, dir, updating)
}

func (a *App) installFullClient(card GameCard, dir string, updating bool) error {
	a.progress(card.Game, "download", 0)
	tmp, err := os.CreateTemp("", "luxview-*.zip")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	if err := a.downloadWithRetry(card.Game, card.DownloadURL, tmpPath); err != nil {
		return fmt.Errorf("falha no download após 3 tentativas: %w", err)
	}

	a.progress(card.Game, "extract", 0)
	restoreSettings := a.backupGameSettings(card, updating)
	if err := unzip(tmpPath, dir, func(done, count int) {
		if count > 0 {
			a.progress(card.Game, "extract", int(float64(done)/float64(count)*100))
		}
	}); err != nil {
		if restoreSettings != nil {
			restoreSettings()
		}
		return fmt.Errorf("falha ao extrair: %w", err)
	}
	if restoreSettings != nil {
		restoreSettings()
	}
	return a.finishClientInstall(card, dir, updating)
}

func (a *App) downloadAndExtractZip(card GameCard, url, dir string, backupSettings bool) error {
	tmp, err := os.CreateTemp("", "luxview-*.zip")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	if err := a.downloadWithRetry(card.Game, url, tmpPath); err != nil {
		return err
	}
	restoreSettings := a.backupGameSettings(card, backupSettings)
	if err := unzip(tmpPath, dir, func(done, count int) {
		if count > 0 {
			a.progress(card.Game, "extract", int(float64(done)/float64(count)*100))
		}
	}); err != nil {
		if restoreSettings != nil {
			restoreSettings()
		}
		return err
	}
	if restoreSettings != nil {
		restoreSettings()
	}
	return nil
}

func (a *App) finishClientInstall(card GameCard, dir string, updating bool) error {
	game := normalizeGameID(card.Game)
	if spec, ok := launchSpecForGame(game); ok && !clientFilesReady(dir, game, spec) {
		return fmt.Errorf("client extraído incompleto — arquivos obrigatórios não encontrados")
	}
	if !updating {
		a.applyDefaultDisplay(card)
	}
	if err := saveInstalledClientHash(card.AppID, card.ClientHash); err != nil {
		return fmt.Errorf("client instalado, mas não gravei a versão local: %w", err)
	}
	if err := saveInstalledBaseHash(card.AppID, card.BaseHash); err != nil {
		return fmt.Errorf("client instalado, mas não gravei o hash do zip base: %w", err)
	}
	a.progress(card.Game, "done", 100)
	return nil
}

func (a *App) ensureClientBaseCache(card GameCard) (string, error) {
	cachePath, err := clientBaseCachePath(card.Game, card.BaseHash)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(cachePath); err == nil && info.Size() > 0 {
		return cachePath, nil
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return "", err
	}
	part := cachePath + ".part"
	a.progress(card.Game, "download", 0)
	if err := a.downloadWithRetry(card.Game, card.BaseURL, part); err != nil {
		_ = os.Remove(part)
		return "", fmt.Errorf("falha no download do client após 3 tentativas: %w", err)
	}
	if err := os.Rename(part, cachePath); err != nil {
		_ = os.Remove(part)
		return "", err
	}
	return cachePath, nil
}

func (a *App) downloadWithRetry(game, url, dest string) error {
	var dlErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if dlErr = a.downloadURL(game, url, dest); dlErr == nil {
			return nil
		}
		if attempt < 3 {
			a.progressMsg(game, "download", -1, fmt.Sprintf("conexão caiu, tentando de novo (%d/3)…", attempt))
			time.Sleep(2 * time.Second)
		}
	}
	return dlErr
}

func (a *App) downloadURL(game, url, path string) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := a.dl.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	pw := &progressWriter{a: a, game: game, total: resp.ContentLength}
	if _, err := io.Copy(out, io.TeeReader(resp.Body, pw)); err != nil {
		return err
	}
	return nil
}
