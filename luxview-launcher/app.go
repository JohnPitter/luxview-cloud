package main

import (
	"archive/zip"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// appVersion is shown in the UI.
const appVersion = "v1.31"

// Version exposes the build tag to the frontend.
func (a *App) Version() string { return appVersion }

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
	AuthHost    string `json:"auth_host"`
	Installed   bool   `json:"installed"` // computed locally
}

// launchSpec tells the launcher how to authenticate and start an installed game,
// replacing the original (SoftNyx) launcher entirely.
type launchSpec struct {
	clientDir   string // playable client dir, relative to install root (zip layout)
	gameExe     string // game executable, relative to clientDir
	settingsINI string // Serious Engine settings file, relative to clientDir
	regHKCU     string // HKCU key whose RootDir points at the client dir
	regHKLM     string // HKLM key whose Location/Version the game reads (needs admin)
	loginPath   string // web auth path (GET user + hex-pass -> token)
	processName string // running game process image name (for "is running" checks)
}

var launchSpecs = map[string]launchSpec{
	"rakion": {
		clientDir:   "client",
		gameExe:     `Bin\load.bin`, // RakionLauncher (.NET): o driver invisível o desempacota e roda sem o diálogo
		settingsINI: `Scripts\PersistentSymbols.ini`,
		regHKCU:     `Software\Softnyx\Rakion`,
		regHKLM:     `SOFTWARE\Softnyx\Rakion`,
		loginPath:   "/launcherlogin.php",
		processName: "rakion.bin",
	},
}

// IsGameRunning reports whether the game's process is currently running, so the
// UI can keep the Play button disabled while you're in-game.
func (a *App) IsGameRunning(game string) bool {
	spec, ok := launchSpecs[game]
	if !ok || spec.processName == "" {
		return false
	}
	return gameProcessRunning(spec.processName)
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
	_, err = os.Stat(filepath.Join(dir, spec.clientDir, spec.gameExe))
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

// Login authenticates against the game's web auth (replacing the original
// launcher's login). Returns the auth token on success.
func (a *App) Login(card GameCard, user, pass string) (string, error) {
	spec, ok := launchSpecs[card.Game]
	if !ok {
		return "", fmt.Errorf("jogo não suportado: %s", card.Game)
	}
	if user == "" || pass == "" {
		return "", fmt.Errorf("informe usuário e senha")
	}
	if card.AuthHost == "" {
		return "", fmt.Errorf("servidor sem host de login configurado")
	}
	// Web auth expects the password hex-encoded (matches the game's own scheme).
	passHex := hex.EncodeToString([]byte(pass))
	u := fmt.Sprintf("https://%s%s?user=%s&pass=%s", card.AuthHost, spec.loginPath,
		url.QueryEscape(user), passHex)
	resp, err := a.client.Get(u)
	if err != nil {
		return "", fmt.Errorf("não consegui contatar o servidor: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	out := strings.TrimSpace(string(body))
	if out == "" {
		return "", fmt.Errorf("resposta vazia do servidor")
	}
	if strings.HasPrefix(out, "[Error]") {
		return "", fmt.Errorf("usuário ou senha incorretos")
	}
	if strings.Contains(strings.ToLower(out), "offline") {
		return "", fmt.Errorf("servidor offline")
	}
	return out, nil // token (sha1)
}

// Play authenticates then launches the game directly (no original launcher),
// passing the SoftNyx-style args: <user> <hex-pass> <token>.
func (a *App) Play(card GameCard, user, pass string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("o jogo só roda no Windows")
	}
	spec, ok := launchSpecs[card.Game]
	if !ok {
		return fmt.Errorf("jogo não suportado: %s", card.Game)
	}
	// Login() validates the credentials against the web auth (launcherlogin.php).
	// We don't forward the long web token to the game: the 3rd arg is a short auth
	// ticket and a 40-char token corrupts the client's login packet, making the
	// world report "ID doesn't exist". A short ticket works (broker is a stub; the
	// world only checks the user/hex-pass we pass as args 1 and 2).
	if _, err := a.Login(card, user, pass); err != nil {
		return err
	}
	dir, err := installDir(card.AppID)
	if err != nil {
		return err
	}
	clientDir := filepath.Join(dir, spec.clientDir)
	exePath := filepath.Join(clientDir, spec.gameExe)
	if _, err := os.Stat(exePath); err != nil {
		return fmt.Errorf("jogo não encontrado — instale primeiro")
	}

	a.ensureRegistry(spec, clientDir)

	passHex := hex.EncodeToString([]byte(pass))

	// BYPASS do diálogo "Window Mode / FullScreen": o load.bin é o RakionLauncher
	// (.NET, MPRESS-packed) que mostra o diálogo. Em vez de rodá-lo normal, um driver
	// (PowerShell 32-bit) desempacota o load.bin, instancia o Form1 dele INVISÍVEL
	// com o modo escolhido pré-selecionado, e roda a pipeline ORIGINAL (login +
	// decrypt config.xfs + lança rakion.bin + patches do GameGuard) — sem o diálogo.
	windowed := true
	if s, err := a.GetSettings(card); err == nil {
		windowed = !s.Fullscreen
		if windowed {
			go frameGameWindow(int32(s.ScreenWidth), int32(s.ScreenHeight))
		}
	}
	if err := invokeRakionDriver(clientDir, user, passHex, windowed); err != nil {
		return err
	}
	return nil
}

// ensureRegistry points the SoftNyx registry keys at the install dir. HKCU never
// needs admin; HKLM (Location/Version) does, so it's only attempted (best effort)
// when missing/wrong — on a machine that already ran the game it's a no-op.
func (a *App) ensureRegistry(spec launchSpec, clientDir string) {
	if spec.regHKCU != "" {
		setHKCURootDir(spec.regHKCU, clientDir+`\`) // silencioso, sem admin
	}
	if spec.regHKLM != "" && !hklmLocationOK(spec.regHKLM, clientDir) {
		_ = setHKLMElevated(spec.regHKLM, clientDir) // reg import oculto, sem prompt
	}
}

// OpenInstallFolder reveals the install dir in the OS file manager.

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

// GameSettings are the player-facing options edited in PersistentSymbols.ini.
type GameSettings struct {
	ScreenWidth      int     `json:"screen_width"`
	ScreenHeight     int     `json:"screen_height"`
	Fullscreen       bool    `json:"fullscreen"`
	MouseSensitivity float64 `json:"mouse_sensitivity"`
	InvertMouse      bool    `json:"invert_mouse"`
	MouseAccel       bool    `json:"mouse_accel"`
	SoundVolume      float64 `json:"sound_volume"`
	MusicVolume      float64 `json:"music_volume"`
	Gamma            float64 `json:"gamma"`
}

func defaultSettings() GameSettings {
	return GameSettings{
		ScreenWidth: 1920, ScreenHeight: 1080, Fullscreen: false,
		MouseSensitivity: 1.5, InvertMouse: false, MouseAccel: true,
		SoundVolume: 0.8, MusicVolume: 0.6, Gamma: 1,
	}
}

func (a *App) iniPath(card GameCard) (string, error) {
	spec, ok := launchSpecs[card.Game]
	if !ok {
		return "", fmt.Errorf("jogo não suportado: %s", card.Game)
	}
	if spec.settingsINI == "" {
		return "", fmt.Errorf("este jogo não tem opções editáveis")
	}
	dir, err := installDir(card.AppID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, spec.clientDir, spec.settingsINI), nil
}

// GetSettings reads the current options from the game's settings file.
func (a *App) GetSettings(card GameCard) (GameSettings, error) {
	s := defaultSettings()
	p, err := a.iniPath(card)
	if err != nil {
		return s, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return s, fmt.Errorf("instale o jogo primeiro")
	}
	c := string(b)
	if v, ok := symbolValue(c, "m_pixScreenWidth"); ok {
		s.ScreenWidth = atoiOr(v, s.ScreenWidth)
	}
	if v, ok := symbolValue(c, "m_pixScreenHeight"); ok {
		s.ScreenHeight = atoiOr(v, s.ScreenHeight)
	}
	if v, ok := symbolValue(c, "m_bActiveFullScreen"); ok {
		s.Fullscreen = v == "1"
	}
	if v, ok := symbolValue(c, "inp_fMouseSensitivity"); ok {
		s.MouseSensitivity = floatOr(v, s.MouseSensitivity)
	}
	if v, ok := symbolValue(c, "inp_bInvertMouse"); ok {
		s.InvertMouse = v == "1"
	}
	if v, ok := symbolValue(c, "inp_bAllowMouseAcceleration"); ok {
		s.MouseAccel = v == "1"
	}
	if v, ok := symbolValue(c, "snd_fSoundVolume"); ok {
		s.SoundVolume = floatOr(v, s.SoundVolume)
	}
	if v, ok := symbolValue(c, "snd_fMusicVolume"); ok {
		s.MusicVolume = floatOr(v, s.MusicVolume)
	}
	if v, ok := symbolValue(c, "gfx_fGamma"); ok {
		s.Gamma = floatOr(v, s.Gamma)
	}
	return s, nil
}

// SaveSettings writes the options back into the game's settings file.
func (a *App) SaveSettings(card GameCard, s GameSettings) error {
	p, err := a.iniPath(card)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("instale o jogo primeiro")
	}
	c := string(b)
	c = setSymbol(c, "m_pixScreenWidth", strconv.Itoa(s.ScreenWidth))
	c = setSymbol(c, "m_pixScreenHeight", strconv.Itoa(s.ScreenHeight))
	c = setSymbol(c, "m_bActiveFullScreen", boolIdx(s.Fullscreen))
	c = setSymbol(c, "inp_fMouseSensitivity", ftoa(s.MouseSensitivity))
	c = setSymbol(c, "inp_bInvertMouse", boolIdx(s.InvertMouse))
	c = setSymbol(c, "inp_bAllowMouseAcceleration", boolIdx(s.MouseAccel))
	c = setSymbol(c, "snd_fSoundVolume", ftoa(s.SoundVolume))
	c = setSymbol(c, "snd_fMusicVolume", ftoa(s.MusicVolume))
	c = setSymbol(c, "gfx_fGamma", ftoa(s.Gamma))
	// The file may be locked read-only (below / by the client); make it writable.
	_ = os.Chmod(p, 0o644)
	if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
		return fmt.Errorf("falha ao salvar as opções: %w", err)
	}
	// Lock it read-only so the game can't overwrite our settings on exit (the
	// Serious Engine persists its own display mode otherwise — losing our choice).
	_ = os.Chmod(p, 0o444)
	return nil
}

func symbolValue(content, name string) (string, bool) {
	re := regexp.MustCompile(regexp.QuoteMeta(name) + `=\((?:INDEX|FLOAT)\)([-0-9.eE]+)`)
	m := re.FindStringSubmatch(content)
	if len(m) < 2 {
		return "", false
	}
	return m[1], true
}

func setSymbol(content, name, value string) string {
	re := regexp.MustCompile(`(` + regexp.QuoteMeta(name) + `=\((?:INDEX|FLOAT)\))[-0-9.eE]+`)
	return re.ReplaceAllString(content, `${1}`+value)
}

func atoiOr(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
func floatOr(s string, def float64) float64 {
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return def
}
func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', -1, 32) }
func boolIdx(b bool) string {
	if b {
		return "1"
	}
	return "0"
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
