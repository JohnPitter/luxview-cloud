package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// baseURL is the LuxView platform origin the launcher talks to. Overridable via
// the LUXVIEW_BASE_URL env var (handy for testing against the VPS directly).
func baseURL() string {
	if v := strings.TrimRight(os.Getenv("LUXVIEW_BASE_URL"), "/"); v != "" {
		return v
	}
	return "https://luxview.cloud"
}

// GameCard mirrors the engine's /api/public/games payload, plus local state.
type GameCard struct {
	AppID       string `json:"app_id"`
	Name        string `json:"name"`
	Game        string `json:"game"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	DownloadURL string `json:"download_url"`
	ServerIP    string `json:"server_ip"`
	Installed   bool   `json:"installed"` // computed locally
}

// launchSpec tells the launcher how to start an installed game.
type launchSpec struct {
	subdir  string // playable client dir, relative to the install root (zip layout)
	exe     string // launcher executable, relative to subdir
	regRoot string // optional HKCU registry key whose RootDir must point at the client dir
}

var launchSpecs = map[string]launchSpec{
	"rakion": {subdir: "client", exe: "NyxLauncher.exe", regRoot: `Software\Softnyx\Rakion`},
}

// App is the Wails backend.
type App struct {
	ctx    context.Context
	client *http.Client // catálogo (JSON pequeno)
	dl     *http.Client // download do client (centenas de MB — sem deadline total)
}

func NewApp() *App {
	return &App{
		client: &http.Client{Timeout: 30 * time.Second},
		// Sem Timeout total (300MB+ pode levar minutos); ainda falha rápido se a
		// conexão/handshake/headers travarem.
		dl: &http.Client{
			Timeout: 0,
			Transport: &http.Transport{
				DialContext:           (&net.Dialer{Timeout: 20 * time.Second}).DialContext,
				TLSHandshakeTimeout:   20 * time.Second,
				ResponseHeaderTimeout: 60 * time.Second,
				ExpectContinueTimeout: 5 * time.Second,
			},
		},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// installsRoot is %APPDATA%/LuxViewLauncher/installs (per-OS config dir).
func installsRoot() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "LuxViewLauncher", "installs"), nil
}

func installDir(appID string) (string, error) {
	root, err := installsRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, appID), nil
}

// GetGames fetches the public catalog and annotates each card with local
// install state.
func (a *App) GetGames() ([]GameCard, error) {
	resp, err := a.client.Get(baseURL() + "/api/public/games")
	if err != nil {
		return nil, fmt.Errorf("não consegui contatar a LuxView Cloud: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catálogo indisponível (HTTP %d)", resp.StatusCode)
	}
	var cards []GameCard
	if err := json.NewDecoder(resp.Body).Decode(&cards); err != nil {
		return nil, fmt.Errorf("resposta inválida do catálogo: %w", err)
	}
	for i := range cards {
		cards[i].Installed = a.isInstalled(cards[i])
	}
	return cards, nil
}

func (a *App) isInstalled(c GameCard) bool {
	dir, err := installDir(c.AppID)
	if err != nil {
		return false
	}
	spec, ok := launchSpecs[c.Game]
	if !ok {
		// Unknown game: consider installed if the folder exists and is non-empty.
		entries, _ := os.ReadDir(dir)
		return len(entries) > 0
	}
	_, err = os.Stat(filepath.Join(dir, spec.subdir, spec.exe))
	return err == nil
}

// IsInstalled is exposed to the frontend for quick checks.
func (a *App) IsInstalled(appID, game string) bool {
	return a.isInstalled(GameCard{AppID: appID, Game: game})
}

// InstallGame downloads the configured client zip and extracts it, emitting
// "install:progress" events ({game, phase, percent}) so the UI can show a bar.
func (a *App) InstallGame(card GameCard) error {
	if card.DownloadURL == "" {
		return fmt.Errorf("este jogo não tem client para download")
	}
	dir, err := installDir(card.AppID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	a.progress(card.Game, "download", 0)
	tmp, err := os.CreateTemp("", "luxview-*.zip")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	// Retry como rede de segurança (queda de conexão no meio do stream).
	var dlErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if dlErr = a.downloadZip(card, tmpPath); dlErr == nil {
			break
		}
		if attempt < 3 {
			a.progressMsg(card.Game, "download", -1, fmt.Sprintf("conexão caiu, tentando de novo (%d/3)…", attempt))
			time.Sleep(2 * time.Second)
		}
	}
	if dlErr != nil {
		return fmt.Errorf("falha no download após 3 tentativas: %w", dlErr)
	}

	a.progress(card.Game, "extract", 0)
	if err := unzip(tmpPath, dir, func(done, count int) {
		if count > 0 {
			a.progress(card.Game, "extract", int(float64(done)/float64(count)*100))
		}
	}); err != nil {
		return fmt.Errorf("falha ao extrair: %w", err)
	}
	a.progress(card.Game, "done", 100)
	return nil
}

// downloadZip streams the client zip to path (one attempt), emitting progress.
func (a *App) downloadZip(card GameCard, path string) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := a.dl.Get(card.DownloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	pw := &progressWriter{a: a, game: card.Game, total: resp.ContentLength}
	if _, err := io.Copy(out, io.TeeReader(resp.Body, pw)); err != nil {
		return err
	}
	return nil
}

// LaunchGame starts the installed game's launcher (Windows only for now).
func (a *App) LaunchGame(card GameCard) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("o launcher dos jogos só roda no Windows")
	}
	dir, err := installDir(card.AppID)
	if err != nil {
		return err
	}
	spec, ok := launchSpecs[card.Game]
	if !ok {
		return fmt.Errorf("jogo não suportado: %s", card.Game)
	}
	clientDir := filepath.Join(dir, spec.subdir)
	exePath := filepath.Join(clientDir, spec.exe)
	if _, err := os.Stat(exePath); err != nil {
		return fmt.Errorf("client não encontrado — instale primeiro")
	}

	// Some legacy clients (Rakion) read their root from the registry.
	if spec.regRoot != "" {
		// reg add "HKCU\<key>" /v RootDir /t REG_SZ /d "<clientDir>\" /f
		_ = exec.Command("reg", "add", `HKCU\`+spec.regRoot,
			"/v", "RootDir", "/t", "REG_SZ", "/d", clientDir+`\`, "/f").Run()
	}

	cmd := exec.Command(exePath)
	cmd.Dir = clientDir
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar o jogo: %w", err)
	}
	return nil
}

// OpenInstallFolder reveals the install dir in the OS file manager.
func (a *App) OpenInstallFolder(appID string) error {
	dir, err := installDir(appID)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return exec.Command("explorer", dir).Start()
	}
	return nil
}

func (a *App) progress(game, phase string, percent int) {
	a.progressMsg(game, phase, percent, "")
}

func (a *App) progressMsg(game, phase string, percent int, detail string) {
	wruntime.EventsEmit(a.ctx, "install:progress", map[string]any{
		"game":    game,
		"phase":   phase,
		"percent": percent,
		"detail":  detail,
	})
}

// progressWriter emits download progress as bytes flow through it.
type progressWriter struct {
	a        *App
	game     string
	total    int64
	written  int64
	lastPct  int
	lastTick int64 // bytes at last MB-based emit (when total unknown)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n := len(b)
	p.written += int64(n)
	if p.total > 0 {
		pct := int(float64(p.written) / float64(p.total) * 100)
		if pct != p.lastPct {
			p.lastPct = pct
			p.a.progress(p.game, "download", pct)
		}
	} else if p.written-p.lastTick >= 1<<20 { // sem Content-Length: emite a cada ~1MB
		p.lastTick = p.written
		mb := float64(p.written) / (1 << 20)
		p.a.progressMsg(p.game, "download", -1, fmt.Sprintf("%.0f MB", mb))
	}
	return n, nil
}

// unzip extracts src into dest, calling onProgress(done, total) per entry.
// Guards against path traversal (zip slip).
func unzip(src, dest string, onProgress func(done, total int)) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	total := len(r.File)
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	for i, f := range r.File {
		target := filepath.Join(dest, f.Name)
		targetAbs, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(targetAbs, destAbs+string(os.PathSeparator)) && targetAbs != destAbs {
			return fmt.Errorf("entrada de zip insegura: %s", f.Name)
		}
		// Some Windows-made zips (this client) use "\" separators and may not flag
		// directory entries via FileInfo — treat trailing-separator names as dirs.
		if f.FileInfo().IsDir() || strings.HasSuffix(f.Name, "/") || strings.HasSuffix(f.Name, `\`) {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()
		if onProgress != nil {
			onProgress(i+1, total)
		}
	}
	return nil
}
