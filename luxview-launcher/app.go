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

// appVersion is shown in the UI. It is a var (not const) so the release CI can
// stamp the real tag via -ldflags "-X main.appVersion=vX.Y"; this is the dev
// fallback when building locally.
var appVersion = "v1.80"

// Version exposes the build tag to the frontend.
func (a *App) Version() string { return appVersion }

// Display modes. The engine settings only store a fullscreen bit, so the launcher
// remembers the windowed/borderless distinction itself (see displayModeFile) and
// applies the right window framing at launch.
const (
	displayFullscreen = "fullscreen" // exclusive fullscreen (overlay-friendly, default)
	displayBorderless = "borderless" // windowed fullscreen: frameless, covers the screen
	displayWindowed   = "windowed"   // titled, draggable, centered window
)

// validDisplayMode normalizes any input to one of the three known modes.
func validDisplayMode(m string) string {
	switch m {
	case displayFullscreen, displayBorderless, displayWindowed:
		return m
	default:
		return displayFullscreen
	}
}

// GameCard mirrors the engine's /api/public/games payload, plus local state.
type GameCard struct {
	AppID           string `json:"app_id"`
	Name            string `json:"name"`
	Game            string `json:"game"`
	DisplayName     string `json:"display_name"`
	Description     string `json:"description"`
	Enabled         bool   `json:"enabled"`
	DownloadURL     string `json:"download_url"`
	PatchURL        string `json:"patch_url"`
	BaseURL         string `json:"base_url"`
	ServerIP        string `json:"server_ip"`
	AuthHost        string `json:"auth_host"`
	ClientHash      string `json:"client_hash"`
	BaseHash        string `json:"base_hash"`
	Installed       bool   `json:"installed"`        // computed locally
	UpdateAvailable bool   `json:"update_available"` // computed locally
}

// launchSpec tells the launcher how to authenticate and start an installed game,
// replacing the original (SoftNyx) launcher entirely.
type launchSpec struct {
	clientDir    string // playable client dir, relative to install root (zip layout)
	gameExe      string // game executable, relative to clientDir
	settingsINI  string // Serious Engine settings file, relative to clientDir
	regHKCU      string // HKCU key whose RootDir points at the client dir
	regHKLM      string // HKLM key whose Location/Version the game reads (needs admin)
	loginPath    string // web auth path (GET user + hex-pass -> token)
	registerPath string // web auth path for self-registration (POST user/pass/email)
	processName  string // running game process image name (for "is running" checks)
}

var launchSpecs = map[string]launchSpec{
	"rakion": {
		clientDir:    "client",
		gameExe:      `Bin\load.bin`, // RakionLauncher (.NET): o driver invisível o desempacota e roda sem o diálogo
		settingsINI:  `Scripts\PersistentSymbols.ini`,
		regHKCU:      `Software\Softnyx\Rakion`,
		regHKLM:      `SOFTWARE\Softnyx\Rakion`,
		loginPath:    "/launcherlogin.php",
		registerPath: "/register.php",
		processName:  "rakion.bin",
	},
	"metin2": {
		clientDir:   "Metin2FullClient",
		gameExe:     "Metin2Distribute.exe",
		settingsINI: "metin2.cfg",
		processName: "Metin2Distribute.exe",
	},
	"tibia": {
		clientDir:   "",
		gameExe:     "otclient.exe",
		processName: "otclient.exe",
	},
	"priston": {
		clientDir:   "",
		gameExe:     "SunnyBPT.exe",
		processName: "SunnyBPT.exe",
	},
	"muemu": {
		clientDir:   "",
		gameExe:     "main.exe",
		processName: "main.exe",
	},
}

func normalizeGameID(raw string) string {
	id := strings.ToLower(strings.TrimSpace(raw))
	id = strings.ReplaceAll(id, "_", "-")
	if id == "tibia" || id == "tibia-canary" || id == "tibia (canary)" || strings.Contains(id, "tibia") {
		return "tibia"
	}
	if id == "openmu" || id == "muemu" || strings.Contains(id, "mu online") {
		return "muemu"
	}
	return id
}

func launchSpecForGame(game string) (launchSpec, bool) {
	spec, ok := launchSpecs[normalizeGameID(game)]
	return spec, ok
}

func resolveGameID(card GameCard) string {
	for _, raw := range []string{card.Game, card.DisplayName, card.Name} {
		id := normalizeGameID(raw)
		if id == "tibia" {
			return id
		}
		if _, ok := launchSpecs[id]; ok {
			return id
		}
	}
	return normalizeGameID(card.Game)
}

// IsGameRunning reports whether the game's process is currently running, so the
// UI can keep the Play button disabled while you're in-game.
func (a *App) IsGameRunning(game string) bool {
	spec, ok := launchSpecForGame(game)
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
	loadLauncherDotEnv()
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
	origin, err := platformOrigin()
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Get(origin + "/api/public/games")
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
		cards[i].Game = resolveGameID(cards[i])
		cards[i].Installed = a.isInstalled(cards[i])
		cards[i].UpdateAvailable = clientNeedsUpdate(cards[i].Installed, cards[i].ClientHash, installedClientHash(cards[i].AppID))
	}
	return cards, nil
}

const clientHashFileName = "luxview-client.hash"

func clientNeedsUpdate(installed bool, catalogHash, localHash string) bool {
	return installed && catalogHash != "" && localHash != catalogHash
}

func installedClientHash(appID string) string {
	dir, err := installDir(appID)
	if err != nil {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(dir, clientHashFileName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func saveInstalledClientHash(appID, hash string) error {
	hash = strings.TrimSpace(hash)
	if appID == "" || hash == "" {
		return nil
	}
	dir, err := installDir(appID)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, clientHashFileName), []byte(hash+"\n"), 0o644)
}

func (a *App) isInstalled(c GameCard) bool {
	game := resolveGameID(c)
	dir, err := installDir(c.AppID)
	if err != nil {
		return false
	}
	spec, ok := launchSpecForGame(game)
	if !ok {
		// Unknown game: consider installed if the folder exists and is non-empty.
		entries, _ := os.ReadDir(dir)
		return len(entries) > 0
	}
	return clientFilesReady(dir, game, spec)
}

func clientFilesReady(installRoot, game string, spec launchSpec) bool {
	if normalizeGameID(game) == "priston" {
		return pristonExecutable(filepath.Join(installRoot, spec.clientDir)) != ""
	}
	if normalizeGameID(game) == "muemu" {
		return muExecutable(filepath.Join(installRoot, spec.clientDir)) != ""
	}
	for _, relativePath := range requiredClientFiles(game, spec) {
		if _, err := os.Stat(filepath.Join(installRoot, spec.clientDir, relativePath)); err != nil {
			return false
		}
	}
	return true
}

func requiredClientFiles(game string, spec launchSpec) []string {
	game = normalizeGameID(game)
	files := []string{spec.gameExe}
	if game == "metin2" {
		files = append(files,
			"python27.dll",
			"SpeedTreeRT.dll",
			filepath.Join("pack", "root.data"),
			"locale.cfg",
		)
	}
	return files
}

// IsInstalled is exposed to the frontend for quick checks.
func (a *App) IsInstalled(appID, game string) bool {
	return a.isInstalled(GameCard{AppID: appID, Game: game})
}

// InstallGame downloads the client. After the first install, a matching base
// hash downloads only the per-server overlay (P-02) instead of the whole zip.
func (a *App) InstallGame(card GameCard) error {
	if card.DownloadURL == "" && !usesSplitClient(card) {
		return fmt.Errorf("este jogo não tem client para download")
	}
	dir, err := installDir(card.AppID)
	if err != nil {
		return err
	}
	if a.IsGameRunning(card.Game) {
		return fmt.Errorf("feche o jogo antes de atualizar o client")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	updating := a.isInstalled(card)
	if usesSplitClient(card) {
		return a.installSplitClient(card, dir, updating)
	}
	return a.installFullClient(card, dir, updating)
}

// applyDefaultDisplay makes a fresh install default to exclusive fullscreen at a
// safe resolution. The shipped Rakion client defaults to windowed @ desktop res,
// which both hides the display-mode distinction and breaks game overlays
// (Discord/NVIDIA only work in exclusive fullscreen on this engine). Players can
// still switch to borderless or windowed in the options.
func (a *App) applyDefaultDisplay(card GameCard) {
	if card.Game != "rakion" {
		return
	}
	s, err := a.GetSettings(card)
	if err != nil {
		return
	}
	s.DisplayMode = displayFullscreen
	if s.ScreenWidth < 640 || s.ScreenHeight < 480 || s.ScreenWidth > 1920 || s.ScreenHeight > 1080 {
		s.ScreenWidth, s.ScreenHeight = 1920, 1080
	}
	if s.Gamma < 0.3 {
		s.Gamma = 1
	}
	_ = a.SaveSettings(card, s)
}

func (a *App) backupGameSettings(card GameCard, updating bool) func() {
	if !updating {
		return nil
	}
	path, err := a.iniPath(card)
	if err != nil {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return func() { _ = os.WriteFile(path, raw, 0o644) }
}

// downloadZip streams the client zip to path (one attempt), emitting progress.
func (a *App) downloadZip(card GameCard, path string) error {
	return a.downloadURL(card.Game, card.DownloadURL, path)
}

// Login authenticates against the game's web auth (replacing the original
// launcher's login). Returns the auth token on success.
func (a *App) Login(card GameCard, user, pass string) (string, error) {
	game := resolveGameID(card)
	spec, ok := launchSpecForGame(game)
	if !ok {
		return "", fmt.Errorf("jogo não suportado: %s", game)
	}
	if user == "" || pass == "" {
		return "", fmt.Errorf("informe usuário e senha")
	}
	if card.AuthHost == "" {
		return "", fmt.Errorf("servidor sem host de login configurado")
	}
	// Web auth expects the password hex-encoded (matches the game's own scheme).
	form := url.Values{}
	form.Set("user", user)
	form.Set("pass", hex.EncodeToString([]byte(pass)))
	resp, err := a.client.PostForm(fmt.Sprintf("https://%s%s", card.AuthHost, spec.loginPath), form)
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

// Register creates a new account on the game's web auth (self-service), so the
// server owner doesn't have to create accounts by hand. The password is sent
// hex-encoded (same scheme as Login) — the server decodes + normalizes it so the
// account works on the next login.
func (a *App) Register(card GameCard, user, pass, email string) error {
	game := resolveGameID(card)
	spec, ok := launchSpecForGame(game)
	if !ok {
		return fmt.Errorf("jogo não suportado: %s", game)
	}
	if spec.registerPath == "" {
		return fmt.Errorf("este servidor não permite cadastro pelo launcher")
	}
	if card.AuthHost == "" {
		return fmt.Errorf("servidor sem host de login configurado")
	}
	user = strings.TrimSpace(user)
	if user == "" || pass == "" {
		return fmt.Errorf("informe usuário e senha")
	}

	form := url.Values{}
	form.Set("user", user)
	form.Set("pass", hex.EncodeToString([]byte(pass)))
	form.Set("email", strings.TrimSpace(email))

	resp, err := a.client.PostForm(fmt.Sprintf("https://%s%s", card.AuthHost, spec.registerPath), form)
	if err != nil {
		return fmt.Errorf("não consegui contatar o servidor: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	out := strings.TrimSpace(string(body))

	if out == "[OK]" {
		return nil
	}
	if msg := strings.TrimSpace(strings.TrimPrefix(out, "[Error]:")); msg != "" && msg != out {
		return fmt.Errorf("%s", msg)
	}
	return fmt.Errorf("falha ao criar conta")
}

// Play authenticates then launches the game directly (no original launcher),
// passing the SoftNyx-style args: <user> <hex-pass> <token>.
func (a *App) Play(card GameCard, user, pass string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("o jogo só roda no Windows")
	}
	game := resolveGameID(card)
	spec, ok := launchSpecForGame(game)
	if !ok {
		return fmt.Errorf("jogo não suportado: %s", game)
	}
	dir, err := installDir(card.AppID)
	if err != nil {
		return err
	}
	clientDir := filepath.Join(dir, spec.clientDir)
	exePath := filepath.Join(clientDir, spec.gameExe)
	if !clientFilesReady(dir, game, spec) {
		return fmt.Errorf("jogo não encontrado — instale primeiro")
	}

	secret, err := loadPlayerSecret()
	if err != nil || secret.Username == "" || secret.Password == "" {
		return fmt.Errorf("entre na conta LuxView")
	}
	charName, vocation := "", ""
	if game == "tibia" {
		charName, vocation = user, pass
	}
	if err := a.ensureGameAccount(card.AppID, secret.Password, charName, vocation); err != nil {
		return err
	}

	if game == "tibia" {
		// Conta Canary = user@luxviewot.com + senha LuxView (mesmo ensureGameAccount).
		// O client deriva o email; jogador só usa user/senha do launcher.
		return launchTibiaExecutable(exePath, clientDir, secret.Username, secret.Password, charName)
	}
	if game == "metin2" {
		return launchExecutable(exePath, clientDir)
	}
	if game == "priston" {
		if resolved := pristonExecutable(clientDir); resolved != "" {
			exePath = resolved
		}
		serverName := strings.TrimSpace(card.DisplayName)
		if serverName == "" {
			serverName = "LuxView"
		}
		if err := writePristonRegistry(card.ServerIP, serverName); err != nil {
			return err
		}
		if err := patchPristonClient(clientDir, card); err != nil {
			return err
		}
		return launchExecutable(exePath, clientDir)
	}
	if game == "muemu" {
		if resolved := muExecutable(clientDir); resolved != "" {
			exePath = resolved
		}
		if err := patchMuClient(clientDir, card); err != nil {
			return err
		}
		return launchMuClient(exePath, clientDir, card.ServerIP, muConnectPort(card))
	}

	if user == "" || pass == "" {
		user = rakionLogin(secret.Username)
		pass = rakionPassword(secret.Password)
	}

	if user == "" || pass == "" {
		return fmt.Errorf("entre na conta LuxView")
	}

	// Login() validates the credentials against the web auth (launcherlogin.php).
	// We don't forward the long web token to the game: the 3rd arg is a short auth
	// ticket and a 40-char token corrupts the client's login packet, making the
	// world report "ID doesn't exist". A short ticket works (broker is a stub; the
	// world only checks the user/hex-pass we pass as args 1 and 2).
	if _, err := a.Login(card, user, pass); err != nil {
		return err
	}
	_ = saveGameLogin(card.AppID, user, pass)

	a.ensureRegistry(spec, clientDir)

	passHex := hex.EncodeToString([]byte(pass))

	// BYPASS do diálogo "Window Mode / FullScreen": o load.bin é o RakionLauncher
	// (.NET, MPRESS-packed) que mostra o diálogo. Em vez de rodá-lo normal, um driver
	// (PowerShell 32-bit) desempacota o load.bin, instancia o Form1 dele INVISÍVEL
	// com o modo escolhido pré-selecionado, e roda a pipeline ORIGINAL (login +
	// decrypt config.xfs + lança rakion.bin + patches do GameGuard) — sem o diálogo.
	//
	// O engine roda WINDOWED tanto para "windowed" quanto para "borderless"; a
	// diferença (janela com título vs. janela sem borda cobrindo a tela) é o
	// enquadramento que o frameGameWindow aplica. Só "fullscreen" roda exclusivo.
	mode := displayFullscreen
	if s, err := a.GetSettings(card); err == nil {
		mode = s.DisplayMode
	}
	windowed := mode != displayFullscreen
	if windowed {
		go frameGameWindow(spec.processName, mode)
	}
	// Atalho global Ctrl+Alt+M pra minimizar/restaurar o jogo (qualquer modo) — útil
	// principalmente em fullscreen exclusivo, onde o Alt+Tab faz device-reset.
	go runMinimizeHotkey(spec.processName)
	// Remove a trava do Alt+Tab/tecla Windows: o keyhook.dll do cliente instala hooks
	// WH_KEYBOARD_LL que engolem essas teclas; com o GameGuard morto, patchamos os
	// hook procs em memória (mov eax,1 -> mov eax,0) e o Alt+Tab volta a funcionar.
	go patchKeyHook(spec.processName)
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
// DisplayMode ("fullscreen"|"borderless"|"windowed") maps to the engine's
// m_bActiveFullScreen bit (fullscreen=1, the other two=0) plus launcher-side window
// framing — the windowed/borderless distinction is persisted by the launcher.
type GameSettings struct {
	ScreenWidth      int     `json:"screen_width"`
	ScreenHeight     int     `json:"screen_height"`
	DisplayMode      string  `json:"display_mode"`
	MouseSensitivity float64 `json:"mouse_sensitivity"`
	InvertMouse      bool    `json:"invert_mouse"`
	MouseAccel       bool    `json:"mouse_accel"`
	SoundVolume      float64 `json:"sound_volume"`
	MusicVolume      float64 `json:"music_volume"`
	Gamma            float64 `json:"gamma"`
}

type Metin2Settings struct {
	ScreenWidth         int     `json:"screen_width"`
	ScreenHeight        int     `json:"screen_height"`
	BPP                 int     `json:"bpp"`
	Frequency           int     `json:"frequency"`
	Windowed            bool    `json:"windowed"`
	SoftwareCursor      bool    `json:"software_cursor"`
	ObjectCulling       bool    `json:"object_culling"`
	Visibility          int     `json:"visibility"`
	MusicVolume         float64 `json:"music_volume"`
	SoundVolume         int     `json:"sound_volume"`
	Gamma               int     `json:"gamma"`
	PreLoadingDelay     int     `json:"pre_loading_delay"`
	DecompressedTexture bool    `json:"decompressed_texture"`
	AlwaysViewName      bool    `json:"always_view_name"`
	ShowRefineDialog    bool    `json:"show_refine_dialog"`
	FogMode             bool    `json:"fog_mode"`
	NightMode           bool    `json:"night_mode"`
	SnowMode            bool    `json:"snow_mode"`
	SnowTexture         bool    `json:"snow_texture"`
	ShowMobLevel        bool    `json:"show_mob_level"`
	ShowMobAIFlag       bool    `json:"show_mob_ai_flag"`
	AutoPickup          bool    `json:"auto_pickup"`
	ExtendedFOV         bool    `json:"extended_fov"`
	EffectLevel         int     `json:"effect_level"`
	PrivateShopLevel    int     `json:"private_shop_level"`
	DropItemLevel       int     `json:"drop_item_level"`
	PetStatus           bool    `json:"pet_status"`
	NPCNameStatus       bool    `json:"npc_name_status"`
	ShowDiceInfo        bool    `json:"show_dice_info"`
	PolyDogMode         bool    `json:"poly_dog_mode"`
	PremiumAffect       bool    `json:"premium_affect"`
	TimeSystem          bool    `json:"time_system"`
	ENBModeStatus       bool    `json:"enb_mode_status"`
	UseDefaultIME       bool    `json:"use_default_ime"`
	SoftwareTiling      int     `json:"software_tiling"`
	ShadowLevel         int     `json:"shadow_level"`
}

func defaultSettings() GameSettings {
	return GameSettings{
		ScreenWidth: 1920, ScreenHeight: 1080, DisplayMode: displayFullscreen,
		MouseSensitivity: 1.5, InvertMouse: false, MouseAccel: true,
		SoundVolume: 0.8, MusicVolume: 0.6, Gamma: 1,
	}
}

// displayModeFile is where the launcher remembers the chosen display mode (the INI
// only has a fullscreen bit, which can't tell windowed from borderless).
func displayModeFile(appID string) (string, error) {
	dir, err := installDir(appID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "display.mode"), nil
}

func readDisplayMode(appID string) string {
	p, err := displayModeFile(appID)
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	switch m := strings.TrimSpace(string(b)); m {
	case displayFullscreen, displayBorderless, displayWindowed:
		return m
	}
	return ""
}

func writeDisplayMode(appID, mode string) {
	p, err := displayModeFile(appID)
	if err != nil {
		return
	}
	_ = os.WriteFile(p, []byte(validDisplayMode(mode)), 0o644)
}

func (a *App) iniPath(card GameCard) (string, error) {
	game := resolveGameID(card)
	spec, ok := launchSpecForGame(game)
	if !ok {
		return "", fmt.Errorf("jogo não suportado: %s", game)
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
	// Display mode: the INI only carries the fullscreen bit. Prefer the launcher's
	// own record (which knows windowed vs. borderless); fall back to the INI bit.
	fs := true
	if v, ok := symbolValue(c, "m_bActiveFullScreen"); ok {
		fs = v == "1"
	}
	if m := readDisplayMode(card.AppID); m != "" {
		s.DisplayMode = m
	} else if fs {
		s.DisplayMode = displayFullscreen
	} else {
		s.DisplayMode = displayWindowed
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
	mode := validDisplayMode(s.DisplayMode)
	// "Janela em tela cheia" (borderless): a Serious Engine desenha o backbuffer 1:1
	// no canto da janela (NÃO estica), então pra preencher a tela sem margens pretas a
	// resolução precisa ser a do cliente de uma janela full-screen. Forçamos isso.
	if mode == displayBorderless {
		if w, h := fillScreenResolution(); w > 0 && h > 0 {
			s.ScreenWidth, s.ScreenHeight = w, h
		}
	}
	c = setSymbol(c, "m_pixScreenWidth", strconv.Itoa(s.ScreenWidth))
	c = setSymbol(c, "m_pixScreenHeight", strconv.Itoa(s.ScreenHeight))
	// Both windowed and borderless run the engine windowed (fullscreen bit = 0);
	// the launcher remembers which one and frames the window accordingly.
	c = setSymbol(c, "m_bActiveFullScreen", boolIdx(mode == displayFullscreen))
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
	// Remember the windowed/borderless choice the INI can't represent.
	writeDisplayMode(card.AppID, mode)
	return nil
}

func defaultMetin2Settings() Metin2Settings {
	return Metin2Settings{
		ScreenWidth: 1366, ScreenHeight: 708, BPP: 32, Frequency: 60,
		Visibility: 3, MusicVolume: 0.6, SoundVolume: 2, Gamma: 1,
		PreLoadingDelay: 20, EffectLevel: 0, PrivateShopLevel: 0,
		DropItemLevel: 0, SoftwareTiling: 0, ShadowLevel: 3,
	}
}

func (a *App) GetMetin2Settings(card GameCard) (Metin2Settings, error) {
	settings := defaultMetin2Settings()
	path, err := a.iniPath(card)
	if err != nil {
		return settings, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return settings, fmt.Errorf("instale o jogo primeiro")
	}
	return parseMetin2Settings(string(content), settings), nil
}

func (a *App) SaveMetin2Settings(card GameCard, settings Metin2Settings) error {
	path, err := a.iniPath(card)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("instale o jogo primeiro")
	}
	updated := writeMetin2Config(string(content), settings)
	_ = os.Chmod(path, 0o644)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("falha ao salvar as configurações do Metin2: %w", err)
	}
	return nil
}

func parseMetin2Settings(content string, settings Metin2Settings) Metin2Settings {
	settings.ScreenWidth = configInt(content, "WIDTH", settings.ScreenWidth)
	settings.ScreenHeight = configInt(content, "HEIGHT", settings.ScreenHeight)
	settings.BPP = configInt(content, "BPP", settings.BPP)
	settings.Frequency = configInt(content, "FREQUENCY", settings.Frequency)
	settings.Windowed = configBool(content, "WINDOWED")
	settings.SoftwareCursor = configBool(content, "SOFTWARE_CURSOR")
	settings.ObjectCulling = configBool(content, "OBJECT_CULLING")
	settings.Visibility = configInt(content, "VISIBILITY", settings.Visibility)
	settings.MusicVolume = configFloat(content, "MUSIC_VOLUME", settings.MusicVolume)
	settings.SoundVolume = configInt(content, "VOICE_VOLUME", settings.SoundVolume)
	settings.Gamma = configInt(content, "GAMMA", settings.Gamma)
	settings.PreLoadingDelay = configInt(content, "PRE_LOADING_DELAY_TIME", settings.PreLoadingDelay)
	settings.DecompressedTexture = configBool(content, "DECOMPRESSED_TEXTURE")
	settings.AlwaysViewName = configBool(content, "ALWAYS_VIEW_NAME")
	settings.ShowRefineDialog = configBool(content, "SHOW_REFINE_DIALOG")
	settings.FogMode = configBool(content, "FOG_MODE_ON")
	settings.NightMode = configBool(content, "NIGHT_MODE_ON")
	settings.SnowMode = configBool(content, "SNOW_MODE_ON")
	settings.SnowTexture = configBool(content, "SNOW_TEXTURE_MODE")
	settings.ShowMobLevel = configBool(content, "SHOW_MOBLEVEL")
	settings.ShowMobAIFlag = configBool(content, "SHOW_MOBAIFLAG")
	settings.AutoPickup = configBool(content, "AUTO_PICKUP")
	settings.ExtendedFOV = configBool(content, "EXTENDED_FOV")
	settings.EffectLevel = configInt(content, "EFFECT_LEVEL", settings.EffectLevel)
	settings.PrivateShopLevel = configInt(content, "PRIVATE_SHOP_LEVEL", settings.PrivateShopLevel)
	settings.DropItemLevel = configInt(content, "DROP_ITEM_LEVEL", settings.DropItemLevel)
	settings.PetStatus = configBool(content, "PET_STATUS")
	settings.NPCNameStatus = configBool(content, "NPC_NAME_STATUS")
	settings.ShowDiceInfo = configBool(content, "SHOW_DICEINFO")
	settings.PolyDogMode = configBool(content, "POLY_DOG_MODE")
	settings.PremiumAffect = configBool(content, "PREMIUM_AFFECT")
	settings.TimeSystem = configBool(content, "TIME_SYSTEM")
	settings.ENBModeStatus = configBool(content, "ENB_MODE_STATUS")
	settings.UseDefaultIME = configBool(content, "USE_DEFAULT_IME")
	settings.SoftwareTiling = configInt(content, "SOFTWARE_TILING", settings.SoftwareTiling)
	settings.ShadowLevel = configInt(content, "SHADOW_LEVEL", settings.ShadowLevel)
	return settings
}

func writeMetin2Config(content string, settings Metin2Settings) string {
	values := []struct{ key, value string }{
		{"WIDTH", strconv.Itoa(clamp(settings.ScreenWidth, 800, 7680))},
		{"HEIGHT", strconv.Itoa(clamp(settings.ScreenHeight, 600, 4320))},
		{"FREQUENCY", strconv.Itoa(clamp(settings.Frequency, 30, 240))},
		{"SOFTWARE_CURSOR", boolIdx(settings.SoftwareCursor)},
		{"OBJECT_CULLING", boolIdx(settings.ObjectCulling)},
		{"VISIBILITY", strconv.Itoa(clamp(settings.Visibility, 0, 3))},
		{"MUSIC_VOLUME", ftoa(clampFloat(settings.MusicVolume, 0, 1))},
		{"VOICE_VOLUME", strconv.Itoa(clamp(settings.SoundVolume, 0, 5))},
		{"GAMMA", strconv.Itoa(clamp(settings.Gamma, 0, 3))},
		{"PRE_LOADING_DELAY_TIME", strconv.Itoa(clamp(settings.PreLoadingDelay, 0, 60))},
		{"DECOMPRESSED_TEXTURE", boolIdx(settings.DecompressedTexture)},
		{"WINDOWED", boolIdx(settings.Windowed)},
		{"ALWAYS_VIEW_NAME", boolIdx(settings.AlwaysViewName)},
		{"SHOW_REFINE_DIALOG", boolIdx(settings.ShowRefineDialog)},
		{"FOG_MODE_ON", boolIdx(settings.FogMode)},
		{"NIGHT_MODE_ON", boolIdx(settings.NightMode)},
		{"SNOW_MODE_ON", boolIdx(settings.SnowMode)},
		{"SNOW_TEXTURE_MODE", boolIdx(settings.SnowTexture)},
		{"SHOW_MOBLEVEL", boolIdx(settings.ShowMobLevel)},
		{"SHOW_MOBAIFLAG", boolIdx(settings.ShowMobAIFlag)},
		{"AUTO_PICKUP", boolIdx(settings.AutoPickup)},
		{"EXTENDED_FOV", boolIdx(settings.ExtendedFOV)},
		{"EFFECT_LEVEL", strconv.Itoa(clamp(settings.EffectLevel, 0, 4))},
		{"PRIVATE_SHOP_LEVEL", strconv.Itoa(clamp(settings.PrivateShopLevel, 0, 4))},
		{"DROP_ITEM_LEVEL", strconv.Itoa(clamp(settings.DropItemLevel, 0, 4))},
		{"PET_STATUS", boolIdx(settings.PetStatus)},
		{"NPC_NAME_STATUS", boolIdx(settings.NPCNameStatus)},
		{"SHOW_DICEINFO", boolIdx(settings.ShowDiceInfo)},
		{"POLY_DOG_MODE", boolIdx(settings.PolyDogMode)},
		{"PREMIUM_AFFECT", boolIdx(settings.PremiumAffect)},
		{"TIME_SYSTEM", boolIdx(settings.TimeSystem)},
		{"ENB_MODE_STATUS", boolIdx(settings.ENBModeStatus)},
		{"USE_DEFAULT_IME", boolIdx(settings.UseDefaultIME)},
		{"SOFTWARE_TILING", strconv.Itoa(clamp(settings.SoftwareTiling, 0, 2))},
		{"SHADOW_LEVEL", strconv.Itoa(clamp(settings.ShadowLevel, 0, 3))},
	}
	for _, item := range values {
		content = setConfigValue(content, item.key, item.value)
	}
	return content
}

func configValue(content, key string) string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `[ \t]+([^\r\n \t]+)`)
	match := re.FindStringSubmatch(content)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func configInt(content, key string, fallback int) int {
	if value, err := strconv.Atoi(configValue(content, key)); err == nil {
		return value
	}
	return fallback
}

func configFloat(content, key string, fallback float64) float64 {
	if value, err := strconv.ParseFloat(configValue(content, key), 64); err == nil {
		return value
	}
	return fallback
}

func configBool(content, key string) bool {
	return configInt(content, key, 0) == 1
}

func setConfigValue(content, key, value string) string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `[ \t]+[^\r\n]*`)
	line := key + "\t\t" + value
	if re.MatchString(content) {
		return re.ReplaceAllString(content, line)
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + line + "\n"
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
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

	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	total := len(r.File)
	for i, f := range r.File {
		if err := extractZipEntry(f, dest, destAbs); err != nil {
			return err
		}
		if onProgress != nil {
			onProgress(i+1, total)
		}
	}
	return nil
}

func extractZipEntry(f *zip.File, dest, destAbs string) error {
	target, err := zipEntryTarget(dest, destAbs, f.Name)
	if err != nil {
		return err
	}
	if f.FileInfo().IsDir() || strings.HasSuffix(f.Name, "/") || strings.HasSuffix(f.Name, `\`) {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := openTruncated(target)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, rc)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func zipEntryTarget(dest, destAbs, name string) (string, error) {
	name = strings.ReplaceAll(name, `\`, "/")
	name = strings.TrimPrefix(name, "/")
	parts := strings.Split(name, "/")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return "", fmt.Errorf("entrada de zip insegura: %s", name)
		}
		clean = append(clean, part)
	}
	target := filepath.Join(append([]string{dest}, clean...)...)
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(targetAbs, destAbs+string(os.PathSeparator)) && targetAbs != destAbs {
		return "", fmt.Errorf("entrada de zip insegura: %s", name)
	}
	return target, nil
}

func openTruncated(path string) (*os.File, error) {
	_ = os.Chmod(path, 0o666)
	var last error
	for i := 0; i < 8; i++ {
		out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err == nil {
			return out, nil
		}
		last = err
		time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
		_ = os.Chmod(path, 0o666)
	}
	return nil, fmt.Errorf("arquivo em uso (%s): %w — feche o jogo e tente de novo", filepath.Base(path), last)
}
