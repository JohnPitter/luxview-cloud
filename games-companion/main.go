package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
)

const (
	listenAddr    = ":8080"
	sessionCookie = "gc_session"
)

var (
	managerPassword = envOr("MANAGER_PASSWORD", "admin")
	serverIP        = envOr("SERVER_IP", "")
	sessions        = map[string]time.Time{}
	sessionsMu      sync.Mutex
)

type SelectOption struct {
	Value string
	Label string
}

type ConfigField struct {
	Key         string
	Label       string
	Type        string // "text", "password", "number", "select"
	Options     []SelectOption
	Placeholder string
	Section     string
}

type ConfigSection struct {
	ID     string
	Label  string
	Fields []ConfigField
}

type GameServer struct {
	ID            string
	DisplayName   string
	Icon          string
	ContainerName string
	GamePort      string
	QueryPort     string
	DataDir       string
	ConfigFields  []ConfigField
}

func sel(pairs ...string) []SelectOption {
	opts := make([]SelectOption, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		opts = append(opts, SelectOption{pairs[i], pairs[i+1]})
	}
	return opts
}

var yesNo = sel("true", "Sim", "false", "Não")

var gameServers = []GameServer{
	{
		ID:            "vrising",
		DisplayName:   "V Rising",
		Icon:          "⚔️",
		ContainerName: envOr("VRISING_CONTAINER", "luxview-vrising"),
		GamePort:      envOr("VRISING_GAME_PORT", "27015"),
		QueryPort:     envOr("VRISING_QUERY_PORT", "27016"),
		DataDir:       envOr("VRISING_DATA_DIR", "/vrising-data"),
		ConfigFields: []ConfigField{
			// Servidor
			{Key: "VRISING_SERVER_NAME", Label: "Nome do Servidor", Type: "text", Placeholder: "V Rising Server", Section: "Servidor"},
			{Key: "VRISING_DESCRIPTION", Label: "Descrição", Type: "text", Placeholder: "Opcional", Section: "Servidor"},
			{Key: "VRISING_PASSWORD", Label: "Senha", Type: "password", Placeholder: "Deixe vazio para público", Section: "Servidor"},
			{Key: "VRISING_MAX_USERS", Label: "Máx. Jogadores", Type: "number", Placeholder: "40", Section: "Servidor"},
			{Key: "VRISING_MAX_ADMINS", Label: "Máx. Admins", Type: "number", Placeholder: "4", Section: "Servidor"},
			{Key: "VRISING_SAVE_NAME", Label: "Nome do Save", Type: "text", Placeholder: "world1", Section: "Servidor"},
			{Key: "VRISING_SAVE_COUNT", Label: "Qtd. de Saves", Type: "number", Placeholder: "20", Section: "Servidor"},
			{Key: "VRISING_SAVE_INTERVAL", Label: "Intervalo de Save (s)", Type: "number", Placeholder: "120", Section: "Servidor"},
			{Key: "VRISING_RESET_DAYS_INTERVAL", Label: "Reset a cada N dias (0=off)", Type: "number", Placeholder: "0", Section: "Servidor"},
			{Key: "VRISING_DAY_OF_RESET", Label: "Dia do Reset", Type: "select", Section: "Servidor", Options: sel(
				"Any", "Qualquer", "Monday", "Segunda", "Tuesday", "Terça",
				"Wednesday", "Quarta", "Thursday", "Quinta", "Friday", "Sexta",
				"Saturday", "Sábado", "Sunday", "Domingo",
			)},
			// Modo de Jogo
			{Key: "VRISING_PRESET", Label: "Preset de Jogo", Type: "select", Section: "Modo de Jogo", Options: sel(
				"StandardPvP", "PvP Padrão", "StandardPvE", "PvE Padrão",
				"HardcorePvP", "PvP Hardcore", "SoloPvP", "PvP Solo", "DuoPvP", "PvP Duo",
			)},
			{Key: "VRISING_DIFFICULTY_PRESET", Label: "Preset de Dificuldade", Type: "select", Section: "Modo de Jogo", Options: sel(
				"", "Padrão (do preset)", "Brutal", "Brutal", "Survivalist", "Sobrevivência", "Relaxed", "Relaxado",
			)},
			{Key: "VRGAME_GAME_MODE_TYPE", Label: "Modo de Jogo", Type: "select", Section: "Modo de Jogo", Options: sel("PvP", "PvP", "PvE", "PvE")},
			{Key: "VRGAME_GAME_DIFFICULTY", Label: "Dificuldade", Type: "select", Section: "Modo de Jogo", Options: sel("Normal", "Normal", "Brutal", "Brutal")},
			{Key: "VRGAME_CASTLE_DAMAGE_MODE", Label: "Dano em Castelos", Type: "select", Section: "Modo de Jogo", Options: sel(
				"Never", "Nunca", "Always", "Sempre", "TimeRestricted", "Horário restrito",
			)},
			{Key: "VRGAME_PLAYER_DAMAGE_MODE", Label: "Dano entre Jogadores", Type: "select", Section: "Modo de Jogo", Options: sel(
				"Always", "Sempre", "TimeRestricted", "Horário restrito",
			)},
			{Key: "VRGAME_CASTLE_HEART_DAMAGE_MODE", Label: "Dano ao Coração do Castelo", Type: "select", Section: "Modo de Jogo", Options: sel(
				"CanBeDestroyedByPlayers", "Pode ser destruído",
				"CanBeSeizedOrDestroyedByPlayers", "Pode ser tomado ou destruído",
				"CannotBeDestroyed", "Indestrutível",
			)},
			{Key: "VRGAME_PVP_PROTECTION_MODE", Label: "Proteção PvP", Type: "select", Section: "Modo de Jogo", Options: sel(
				"Disabled", "Desativado", "VeryShort", "Muito curto", "Short", "Curto", "Medium", "Médio",
			)},
			{Key: "VRGAME_DEATH_CONTAINER_PERMISSION", Label: "Loot na morte", Type: "select", Section: "Modo de Jogo", Options: sel(
				"Anyone", "Qualquer um", "KillerOnly", "Apenas quem matou", "OwnerOnly", "Apenas o dono",
			)},
			{Key: "VRGAME_CLAN_SIZE", Label: "Tamanho máx. do Clã", Type: "number", Placeholder: "4", Section: "Modo de Jogo"},
			{Key: "VRGAME_ALLOW_GLOBAL_CHAT", Label: "Chat Global", Type: "select", Section: "Modo de Jogo", Options: yesNo},
			{Key: "VRGAME_ALL_WAYPOINTS_UNLOCKED", Label: "Todos Waypoints desbloqueados", Type: "select", Section: "Modo de Jogo", Options: yesNo},
			{Key: "VRGAME_BLOOD_BOUND_EQUIPMENT", Label: "Equipamento ligado ao sangue", Type: "select", Section: "Modo de Jogo", Options: yesNo},
			{Key: "VRGAME_TELEPORT_BOUND_ITEMS", Label: "Itens prendem teleporte", Type: "select", Section: "Modo de Jogo", Options: yesNo},
			{Key: "VRGAME_CAN_LOOT_ENEMY_CONTAINERS", Label: "Saques em containers inimigos", Type: "select", Section: "Modo de Jogo", Options: yesNo},
			{Key: "VRGAME_FREE_CASTLE_RAID", Label: "Raid gratuita", Type: "select", Section: "Modo de Jogo", Options: yesNo},
			{Key: "VRGAME_FREE_CASTLE_CLAIM", Label: "Reivindicação gratuita", Type: "select", Section: "Modo de Jogo", Options: yesNo},
			{Key: "VRGAME_FREE_CASTLE_DESTROY", Label: "Destruição gratuita", Type: "select", Section: "Modo de Jogo", Options: yesNo},
			{Key: "VRGAME_INACTIVITY_KILL_ENABLED", Label: "Kill por inatividade", Type: "select", Section: "Modo de Jogo", Options: yesNo},
			// Taxas
			{Key: "VRGAME_INVENTORY_STACKS", Label: "Pilhas de Inventário", Type: "number", Placeholder: "1.0", Section: "Taxas"},
			{Key: "VRGAME_DROP_RATE", Label: "Taxa de Drop", Type: "number", Placeholder: "1.0", Section: "Taxas"},
			{Key: "VRGAME_MATERIAL_YIELD", Label: "Rendimento de Materiais", Type: "number", Placeholder: "1.0", Section: "Taxas"},
			{Key: "VRGAME_BLOOD_ESSENCE_YIELD", Label: "Rendimento de Essência", Type: "number", Placeholder: "1.0", Section: "Taxas"},
			{Key: "VRGAME_CRAFT_RATE", Label: "Velocidade de Crafting", Type: "number", Placeholder: "1.0", Section: "Taxas"},
			{Key: "VRGAME_BUILD_COST", Label: "Custo de Construção", Type: "number", Placeholder: "1.0", Section: "Taxas"},
			{Key: "VRGAME_RECIPE_COST", Label: "Custo de Receita", Type: "number", Placeholder: "1.0", Section: "Taxas"},
			{Key: "VRGAME_RESEARCH_COST", Label: "Custo de Pesquisa", Type: "number", Placeholder: "1.0", Section: "Taxas"},
			{Key: "VRGAME_REFINEMENT_RATE", Label: "Velocidade de Refinamento", Type: "number", Placeholder: "1.0", Section: "Taxas"},
			{Key: "VRGAME_REFINEMENT_COST", Label: "Custo de Refinamento", Type: "number", Placeholder: "1.0", Section: "Taxas"},
			{Key: "VRGAME_DISMANTLE_RESOURCE", Label: "Recursos ao Desmontar", Type: "number", Placeholder: "1.0", Section: "Taxas"},
			{Key: "VRGAME_SERVANT_CONVERT_RATE", Label: "Velocidade de Conversão de Servo", Type: "number", Placeholder: "1.0", Section: "Taxas"},
			{Key: "VRGAME_REPAIR_COST", Label: "Custo de Reparo", Type: "number", Placeholder: "1.0", Section: "Taxas"},
			{Key: "VRGAME_BLOOD_DRAIN", Label: "Drenagem de Sangue", Type: "number", Placeholder: "1.0", Section: "Taxas"},
			{Key: "VRGAME_DURABILITY_DRAIN", Label: "Drenagem de Durabilidade", Type: "number", Placeholder: "1.0", Section: "Taxas"},
			// Mundo
			{Key: "VRGAME_DAY_DURATION", Label: "Duração do Dia (s)", Type: "number", Placeholder: "1080", Section: "Mundo"},
			{Key: "VRGAME_DAY_START_HOUR", Label: "Hora de Início do Dia", Type: "number", Placeholder: "9", Section: "Mundo"},
			{Key: "VRGAME_DAY_END_HOUR", Label: "Hora de Fim do Dia", Type: "number", Placeholder: "17", Section: "Mundo"},
			{Key: "VRGAME_BLOOD_MOON_MIN", Label: "Freq. Mín. Lua de Sangue (dias)", Type: "number", Placeholder: "10", Section: "Mundo"},
			{Key: "VRGAME_BLOOD_MOON_MAX", Label: "Freq. Máx. Lua de Sangue (dias)", Type: "number", Placeholder: "18", Section: "Mundo"},
			{Key: "VRGAME_BLOOD_MOON_BUFF", Label: "Bônus da Lua de Sangue", Type: "number", Placeholder: "0.2", Section: "Mundo"},
			{Key: "VRGAME_GARLIC_STRENGTH", Label: "Força do Alho", Type: "number", Placeholder: "1.0", Section: "Mundo"},
			{Key: "VRGAME_HOLY_STRENGTH", Label: "Força Sagrada", Type: "number", Placeholder: "1.0", Section: "Mundo"},
			{Key: "VRGAME_SILVER_STRENGTH", Label: "Força da Prata", Type: "number", Placeholder: "1.0", Section: "Mundo"},
			{Key: "VRGAME_SUN_DAMAGE", Label: "Dano Solar", Type: "number", Placeholder: "1.0", Section: "Mundo"},
			{Key: "VRGAME_CASTLE_DECAY_RATE", Label: "Decaimento do Castelo", Type: "number", Placeholder: "1.0", Section: "Mundo"},
			{Key: "VRGAME_CASTLE_BLOOD_DRAIN", Label: "Drenagem de Sangue do Castelo", Type: "number", Placeholder: "1.0", Section: "Mundo"},
			{Key: "VRGAME_CASTLE_RELOCATION", Label: "Realocação de Castelo", Type: "select", Section: "Mundo", Options: yesNo},
			// Vampiros
			{Key: "VRGAME_VAMPIRE_HEALTH", Label: "Vida do Vampiro", Type: "number", Placeholder: "1.0", Section: "Vampiros"},
			{Key: "VRGAME_VAMPIRE_PHYSICAL_POWER", Label: "Poder Físico", Type: "number", Placeholder: "1.0", Section: "Vampiros"},
			{Key: "VRGAME_VAMPIRE_SPELL_POWER", Label: "Poder Mágico", Type: "number", Placeholder: "1.0", Section: "Vampiros"},
			{Key: "VRGAME_VAMPIRE_RESOURCE_POWER", Label: "Poder de Recurso", Type: "number", Placeholder: "1.0", Section: "Vampiros"},
			{Key: "VRGAME_VAMPIRE_SIEGE_POWER", Label: "Poder de Cerco", Type: "number", Placeholder: "1.0", Section: "Vampiros"},
			{Key: "VRGAME_VAMPIRE_DAMAGE_RECEIVED", Label: "Dano Recebido", Type: "number", Placeholder: "1.0", Section: "Vampiros"},
			{Key: "VRGAME_UNIT_HEALTH", Label: "Vida das Unidades", Type: "number", Placeholder: "1.0", Section: "Vampiros"},
			{Key: "VRGAME_UNIT_POWER", Label: "Poder das Unidades", Type: "number", Placeholder: "1.0", Section: "Vampiros"},
			{Key: "VRGAME_UNIT_LEVEL_INCREASE", Label: "Aumento de Nível das Unidades", Type: "number", Placeholder: "0", Section: "Vampiros"},
			{Key: "VRGAME_VBLOOD_HEALTH", Label: "Vida dos V-Blood", Type: "number", Placeholder: "1.0", Section: "Vampiros"},
			{Key: "VRGAME_VBLOOD_POWER", Label: "Poder dos V-Blood", Type: "number", Placeholder: "1.0", Section: "Vampiros"},
			{Key: "VRGAME_VBLOOD_LEVEL_INCREASE", Label: "Aumento de Nível V-Blood", Type: "number", Placeholder: "0", Section: "Vampiros"},
			// Avançado
			{Key: "VRISING_FPS", Label: "FPS Máximo", Type: "number", Placeholder: "30", Section: "Avançado"},
			{Key: "VRISING_LOWER_FPS_WHEN_EMPTY", Label: "Reduzir FPS quando vazio", Type: "select", Section: "Avançado", Options: yesNo},
			{Key: "VRISING_LOWER_FPS_WHEN_EMPTY_VALUE", Label: "FPS quando vazio", Type: "number", Placeholder: "15", Section: "Avançado"},
			{Key: "VRISING_SECURE", Label: "Servidor seguro (VAC)", Type: "select", Section: "Avançado", Options: yesNo},
			{Key: "VRISING_LIST_ON_STEAM", Label: "Listar no Steam", Type: "select", Section: "Avançado", Options: yesNo},
			{Key: "VRISING_LIST_ON_EOS", Label: "Listar no EOS", Type: "select", Section: "Avançado", Options: yesNo},
			{Key: "VRISING_RCON_ENABLED", Label: "RCON Habilitado", Type: "select", Section: "Avançado", Options: sel("true", "Sim", "", "Não")},
			{Key: "VRISING_RCON_PORT", Label: "Porta RCON", Type: "number", Placeholder: "25575", Section: "Avançado"},
			{Key: "VRISING_RCON_PASSWORD", Label: "Senha RCON", Type: "password", Placeholder: "Senha RCON", Section: "Avançado"},
		},
	},
}

func findServer(id string) *GameServer {
	for i := range gameServers {
		if gameServers[i].ID == id {
			return &gameServers[i]
		}
	}
	return nil
}

func groupSections(gs *GameServer) []ConfigSection {
	var order []string
	byLabel := map[string]*ConfigSection{}
	for _, f := range gs.ConfigFields {
		sec := f.Section
		if sec == "" {
			sec = "Geral"
		}
		if _, ok := byLabel[sec]; !ok {
			id := strings.Map(func(r rune) rune {
				if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
					return r
				}
				if r >= 'A' && r <= 'Z' {
					return r + 32
				}
				return '-'
			}, sec)
			byLabel[sec] = &ConfigSection{ID: id, Label: sec}
			order = append(order, sec)
		}
		byLabel[sec].Fields = append(byLabel[sec].Fields, f)
	}
	out := make([]ConfigSection, 0, len(order))
	for _, l := range order {
		out = append(out, *byLabel[l])
	}
	return out
}

// --- A2S query (Steam server info protocol) ---

type A2SInfo struct {
	Players    int
	MaxPlayers int
}

func queryA2S(addr string) (*A2SInfo, error) {
	conn, err := net.DialTimeout("udp", addr, 3*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second)) //nolint:errcheck

	req := append([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x54}, "Source Engine Query\x00"...)
	if _, err := conn.Write(req); err != nil {
		return nil, err
	}

	buf := make([]byte, 1400)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	buf = buf[:n]

	// challenge response
	if len(buf) >= 9 && buf[4] == 0x41 {
		req2 := append(append([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x54}, "Source Engine Query\x00"...), buf[5:9]...)
		if _, err := conn.Write(req2); err != nil {
			return nil, err
		}
		n, err = conn.Read(buf)
		if err != nil {
			return nil, err
		}
		buf = buf[:n]
	}

	if len(buf) < 6 || buf[4] != 0x49 {
		return nil, fmt.Errorf("unexpected A2S type: %x", buf[4])
	}

	r := bytes.NewReader(buf[5:])
	r.ReadByte() // protocol
	for i := 0; i < 4; i++ {
		for b, _ := r.ReadByte(); b != 0; b, _ = r.ReadByte() {
		} // skip null-terminated string
	}
	r.ReadByte() // appID low
	r.ReadByte() // appID high
	players, _ := r.ReadByte()
	maxPlayers, _ := r.ReadByte()
	return &A2SInfo{Players: int(players), MaxPlayers: int(maxPlayers)}, nil
}

// --- Auth ---

func newToken() string {
	b := make([]byte, 16)
	rand.Read(b) //nolint:errcheck
	return hex.EncodeToString(b)
}

func isAuthed(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	exp, ok := sessions[c.Value]
	if !ok || time.Now().After(exp) {
		delete(sessions, c.Value)
		return false
	}
	return true
}

func auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isAuthed(r) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next(w, r)
	}
}

// --- Config ---

func configFilePath(gs *GameServer) string {
	return filepath.Join(gs.DataDir, "server-config.env")
}

func loadConfig(gs *GameServer) map[string]string {
	cfg := map[string]string{}
	for _, f := range gs.ConfigFields {
		if v := os.Getenv(f.Key); v != "" {
			cfg[f.Key] = v
		}
	}
	file, err := os.Open(configFilePath(gs))
	if err != nil {
		return cfg
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		k, v, ok := strings.Cut(scanner.Text(), "=")
		if !ok {
			continue
		}
		cfg[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return cfg
}

func saveConfig(gs *GameServer, values map[string]string) error {
	var sb strings.Builder
	for _, f := range gs.ConfigFields {
		fmt.Fprintf(&sb, "%s=%s\n", f.Key, values[f.Key])
	}
	return os.WriteFile(configFilePath(gs), []byte(sb.String()), 0644)
}

// --- Docker ---

type Status struct {
	Running      bool   `json:"running"`
	State        string `json:"state"`
	Uptime       string `json:"uptime,omitempty"`
	RestartCount int    `json:"restart_count"`
	Players      int    `json:"players"`
	MaxPlayers   int    `json:"max_players"`
}

func getStatus(ctx context.Context, cli *dockerclient.Client, gs *GameServer) Status {
	info, err := cli.ContainerInspect(ctx, gs.ContainerName)
	if err != nil {
		return Status{State: "not found"}
	}
	s := Status{Running: info.State.Running, State: info.State.Status, RestartCount: info.RestartCount}
	if info.State.Running {
		if started, err := time.Parse(time.RFC3339Nano, info.State.StartedAt); err == nil {
			s.Uptime = formatDuration(time.Since(started))
		}
		if gs.QueryPort != "" {
			if a2s, err := queryA2S("host.docker.internal:" + gs.QueryPort); err == nil {
				s.Players = a2s.Players
				s.MaxPlayers = a2s.MaxPlayers
			}
		}
	}
	return s
}

func restartContainer(ctx context.Context, cli *dockerclient.Client, name string) error {
	t := 30
	return cli.ContainerRestart(ctx, name, container.StopOptions{Timeout: &t})
}

// --- Main ---

func main() {
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal("docker client:", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", loginPageHandler)
	mux.HandleFunc("POST /login", loginSubmitHandler)
	mux.HandleFunc("POST /logout", logoutHandler)
	mux.HandleFunc("GET /", auth(indexHandler))
	mux.HandleFunc("GET /servers/{id}", auth(serverPageHandler))
	mux.HandleFunc("GET /api/servers/{id}/status", auth(apiStatusHandler(cli)))
	mux.HandleFunc("GET /api/servers/{id}/config", auth(apiGetConfigHandler))
	mux.HandleFunc("POST /api/servers/{id}/config", auth(apiSetConfigHandler(cli)))
	mux.HandleFunc("POST /api/servers/{id}/restart", auth(apiRestartHandler(cli)))

	log.Println("games-companion listening on", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, mux))
}

// --- Handlers ---

func loginPageHandler(w http.ResponseWriter, r *http.Request) {
	if isAuthed(r) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	renderPage(w, loginTmpl, map[string]any{"Failed": r.URL.Query().Get("failed") == "1"})
}

func loginSubmitHandler(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("password") != managerPassword {
		http.Redirect(w, r, "/login?failed=1", http.StatusFound)
		return
	}
	token := newToken()
	sessionsMu.Lock()
	sessions[token] = time.Now().Add(8 * time.Hour)
	sessionsMu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/", http.StatusFound)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		sessionsMu.Lock()
		delete(sessions, c.Value)
		sessionsMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, MaxAge: -1, Path: "/"})
	http.Redirect(w, r, "/login", http.StatusFound)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, indexTmpl, map[string]any{"Servers": gameServers})
}

func serverPageHandler(w http.ResponseWriter, r *http.Request) {
	gs := findServer(r.PathValue("id"))
	if gs == nil {
		http.NotFound(w, r)
		return
	}
	renderPage(w, serverTmpl, map[string]any{
		"Server":   gs,
		"Sections": groupSections(gs),
		"Config":   loadConfig(gs),
		"ServerIP": serverIP,
	})
}

func apiStatusHandler(cli *dockerclient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gs := findServer(r.PathValue("id"))
		if gs == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(getStatus(r.Context(), cli, gs))
	}
}

func apiGetConfigHandler(w http.ResponseWriter, r *http.Request) {
	gs := findServer(r.PathValue("id"))
	if gs == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loadConfig(gs))
}

func apiSetConfigHandler(cli *dockerclient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gs := findServer(r.PathValue("id"))
		if gs == nil {
			http.NotFound(w, r)
			return
		}
		var values map[string]string
		if err := json.NewDecoder(r.Body).Decode(&values); err != nil {
			http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
			return
		}
		if err := saveConfig(gs, values); err != nil {
			http.Error(w, `{"error":"failed to save"}`, http.StatusInternalServerError)
			return
		}
		go func() {
			time.Sleep(300 * time.Millisecond)
			restartContainer(context.Background(), cli, gs.ContainerName)
		}()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"message":"saved, restarting"}`)
	}
}

func apiRestartHandler(cli *dockerclient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gs := findServer(r.PathValue("id"))
		if gs == nil {
			http.NotFound(w, r)
			return
		}
		if err := restartContainer(r.Context(), cli, gs.ContainerName); err != nil {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"message":"restarting"}`)
	}
}

// --- Helpers ---

func renderPage(w http.ResponseWriter, src string, data any) {
	tmpl := template.Must(template.New("").Funcs(template.FuncMap{
		"eq": func(a, b string) bool { return a == b },
	}).Parse(baseHTML + src))
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		log.Println("render error:", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h, m, s := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// --- Templates ---

const baseHTML = `{{define "base"}}<!DOCTYPE html>
<html lang="pt-BR">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Games Companion — Luxview</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{background:#09090b;color:#fafafa;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;min-height:100vh;font-size:14px;-webkit-font-smoothing:antialiased}
::-webkit-scrollbar{width:5px;height:5px}
::-webkit-scrollbar-track{background:transparent}
::-webkit-scrollbar-thumb{background:#3f3f46;border-radius:9999px}
a{color:inherit;text-decoration:none}
/* floating pill nav */
.nav{position:fixed;top:20px;left:50%;transform:translateX(-50%);display:flex;align-items:center;gap:16px;height:48px;padding:0 20px;background:rgba(9,9,11,.85);backdrop-filter:blur(16px);-webkit-backdrop-filter:blur(16px);border:1px solid rgba(63,63,70,.5);border-radius:9999px;box-shadow:0 20px 60px rgba(0,0,0,.5);z-index:50;white-space:nowrap}
.nav-logo{width:32px;height:32px;background:linear-gradient(135deg,#fbbf24,#f59e0b);border-radius:8px;display:flex;align-items:center;justify-content:center;font-size:16px;flex-shrink:0;box-shadow:0 0 15px rgba(251,191,36,.25)}
.nav-title{font-size:13px;font-weight:600;color:#fafafa}
.nav-sep{width:1px;height:20px;background:rgba(63,63,70,.7)}
.btn{display:inline-flex;align-items:center;gap:6px;border-radius:10px;padding:7px 16px;font-size:13px;font-weight:500;cursor:pointer;transition:all .2s;border:none;outline:none}
.btn-ghost{background:transparent;border:1px solid rgba(63,63,70,.6);color:#a1a1aa}
.btn-ghost:hover{border-color:rgba(113,113,122,.8);color:#fafafa;background:rgba(39,39,42,.4)}
.btn-amber{background:#fbbf24;color:#000;font-weight:600}
.btn-amber:hover{background:#f59e0b;box-shadow:0 0 20px rgba(251,191,36,.3)}
.btn-amber:disabled{opacity:.45;cursor:not-allowed;box-shadow:none}
.btn-danger{background:rgba(239,68,68,.08);border:1px solid rgba(239,68,68,.2);color:#f87171}
.btn-danger:hover{background:rgba(239,68,68,.15);border-color:rgba(239,68,68,.35)}
.btn-danger:disabled{opacity:.45;cursor:not-allowed}
/* glass card */
.card{background:rgba(24,24,27,.5);backdrop-filter:blur(12px);-webkit-backdrop-filter:blur(12px);border:1px solid rgba(63,63,70,.5);border-radius:16px;padding:24px}
.card-hover{transition:all .2s}
.card-hover:hover{border-color:rgba(113,113,122,.5);box-shadow:0 0 0 1px rgba(63,63,70,.3),0 8px 32px rgba(0,0,0,.4);transform:translateY(-1px)}
/* badges */
.badge{display:inline-flex;align-items:center;gap:5px;padding:3px 10px;border-radius:9999px;font-size:11px;font-weight:500}
.badge-on{background:rgba(52,211,153,.08);border:1px solid rgba(52,211,153,.2);color:#34d399}
.badge-off{background:rgba(248,113,113,.08);border:1px solid rgba(248,113,113,.2);color:#f87171}
.dot{width:5px;height:5px;border-radius:50%;background:currentColor}
/* section label */
.section-label{font-size:10px;font-weight:600;text-transform:uppercase;letter-spacing:.1em;color:#52525b;margin-bottom:16px}
/* inputs */
label{display:block;font-size:12px;color:#71717a;margin-bottom:5px;font-weight:500}
input,select{width:100%;background:rgba(9,9,11,.7);border:1px solid rgba(63,63,70,.6);border-radius:10px;padding:8px 12px;color:#fafafa;font-size:13px;outline:none;transition:border-color .2s}
input:focus,select:focus{border-color:#fbbf24;box-shadow:0 0 0 2px rgba(251,191,36,.1)}
select option{background:#18181b}
/* separator */
.sep{border:none;border-top:1px solid rgba(63,63,70,.4);margin:0}
/* toast */
.toast{position:fixed;top:80px;left:50%;transform:translateX(-50%);background:rgba(24,24,27,.9);backdrop-filter:blur(12px);border:1px solid rgba(63,63,70,.5);border-radius:12px;padding:12px 20px;font-size:13px;opacity:0;transition:opacity .25s;pointer-events:none;z-index:999;white-space:nowrap;box-shadow:0 8px 32px rgba(0,0,0,.4)}
.toast.show{opacity:1}
.toast.ok{border-color:rgba(52,211,153,.3);color:#34d399}
.toast.err{border-color:rgba(248,113,113,.3);color:#f87171}
</style>
</head>
<body>
<nav class="nav">
  <a href="/" style="display:flex;align-items:center;gap:10px">
    <div class="nav-logo">🎮</div>
    <span class="nav-title">Games Companion</span>
  </a>
  <div class="nav-sep"></div>
  <span style="font-size:12px;color:#52525b">luxview.cloud</span>
  <div class="nav-sep"></div>
  <form method="POST" action="/logout" style="margin:0">
    <button class="btn btn-ghost" type="submit" style="padding:5px 12px;font-size:12px">Sair</button>
  </form>
</nav>
<main style="max-width:960px;margin:0 auto;padding:88px 20px 40px">{{block "content" .}}{{end}}</main>
<div class="toast" id="toast"></div>
<script>
function toast(msg,type){const t=document.getElementById('toast');t.textContent=msg;t.className='toast show '+(type||'');clearTimeout(t._t);t._t=setTimeout(()=>t.className='toast',3500)}
</script>
</body></html>
{{end}}`

const loginTmpl = `{{define "content"}}
<div style="min-height:calc(100vh - 88px);display:flex;align-items:center;justify-content:center;padding-top:0">
  <div style="width:100%;max-width:380px">
    <div style="text-align:center;margin-bottom:32px">
      <div style="width:56px;height:56px;background:linear-gradient(135deg,#fbbf24,#f59e0b);border-radius:14px;display:flex;align-items:center;justify-content:center;font-size:26px;margin:0 auto 16px;box-shadow:0 0 30px rgba(251,191,36,.25)">🎮</div>
      <h1 style="font-size:22px;font-weight:700;margin-bottom:6px">Games Companion</h1>
      <p style="font-size:13px;color:#71717a">Acesso restrito ao gerenciamento de servidores</p>
    </div>
    <div class="card">
      {{if .Failed}}<div style="background:rgba(239,68,68,.08);border:1px solid rgba(239,68,68,.2);border-radius:10px;padding:10px 14px;font-size:13px;color:#f87171;margin-bottom:18px">Senha incorreta. Tente novamente.</div>{{end}}
      <form method="POST" action="/login">
        <label style="margin-bottom:6px">Senha de acesso</label>
        <input type="password" name="password" autofocus placeholder="••••••••" style="margin-bottom:18px">
        <button class="btn btn-amber" type="submit" style="width:100%;justify-content:center;padding:10px">Entrar</button>
      </form>
    </div>
  </div>
</div>
{{end}}`

const indexTmpl = `{{define "content"}}
<div style="margin-bottom:28px">
  <h1 style="font-size:22px;font-weight:700">Servidores</h1>
  <p style="font-size:13px;color:#71717a;margin-top:4px">{{len .Servers}} servidor(es) gerenciado(s)</p>
</div>
<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(290px,1fr));gap:16px">
  {{range .Servers}}
  <a href="/servers/{{.ID}}">
    <div class="card card-hover" style="cursor:pointer">
      <div style="display:flex;align-items:center;gap:14px;margin-bottom:20px">
        <div style="width:48px;height:48px;background:rgba(251,191,36,.08);border:1px solid rgba(251,191,36,.15);border-radius:12px;display:flex;align-items:center;justify-content:center;font-size:22px;flex-shrink:0">{{.Icon}}</div>
        <div>
          <div style="font-size:16px;font-weight:600;margin-bottom:2px">{{.DisplayName}}</div>
          <div style="font-size:11px;color:#52525b;font-family:monospace">{{.ContainerName}}</div>
        </div>
      </div>
      <hr class="sep" style="margin-bottom:16px">
      <div style="display:flex;align-items:center;justify-content:space-between">
        <span class="badge badge-off" id="badge-{{.ID}}"><span class="dot"></span>Verificando...</span>
        <span style="font-size:12px;color:#52525b;font-family:monospace">:{{.GamePort}}</span>
      </div>
    </div>
  </a>
  {{end}}
</div>
<script>
(async()=>{
  {{range .Servers}}
  try{
    const r=await fetch('/api/servers/{{.ID}}/status'),s=await r.json();
    const b=document.getElementById('badge-{{.ID}}');
    if(s.running){
      b.className='badge badge-on';
      b.innerHTML='<span class="dot"></span>Online'+(s.uptime?' · '+s.uptime:'')+(s.max_players?' · '+s.players+'/'+s.max_players:'');
    }else{b.className='badge badge-off';b.innerHTML='<span class="dot"></span>'+(s.state||'Offline');}
  }catch(e){}
  {{end}}
})();
</script>
{{end}}`

const serverTmpl = `{{define "content"}}
<div style="margin-bottom:8px">
  <a href="/" style="font-size:13px;color:#52525b;display:inline-flex;align-items:center;gap:4px;transition:color .2s" onmouseover="this.style.color='#a1a1aa'" onmouseout="this.style.color='#52525b'">← Servidores</a>
</div>
<div style="display:flex;align-items:flex-start;justify-content:space-between;flex-wrap:wrap;gap:16px;margin-bottom:24px">
  <div style="display:flex;align-items:center;gap:16px">
    <div style="width:56px;height:56px;background:rgba(251,191,36,.08);border:1px solid rgba(251,191,36,.15);border-radius:14px;display:flex;align-items:center;justify-content:center;font-size:26px;flex-shrink:0">{{.Server.Icon}}</div>
    <div>
      <h1 style="font-size:22px;font-weight:700;margin-bottom:8px">{{.Server.DisplayName}}</h1>
      <div style="display:flex;align-items:center;gap:10px;flex-wrap:wrap">
        <span class="badge badge-off" id="status-badge"><span class="dot"></span>Verificando...</span>
        <span style="font-size:12px;color:#71717a" id="players-text"></span>
      </div>
    </div>
  </div>
  <button class="btn btn-danger" id="restart-btn" onclick="restartServer()">↺ Reiniciar</button>
</div>

<div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-bottom:20px">
  <div class="card">
    <div class="section-label">Status</div>
    <div style="display:flex;flex-direction:column;gap:12px">
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="color:#71717a;font-size:13px">Uptime</span>
        <span style="font-family:monospace;font-size:13px;color:#a1a1aa" id="s-uptime">—</span>
      </div>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="color:#71717a;font-size:13px">Estado</span>
        <span style="font-family:monospace;font-size:13px;color:#a1a1aa" id="s-state">—</span>
      </div>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="color:#71717a;font-size:13px">Reinicializações</span>
        <span style="font-family:monospace;font-size:13px;color:#a1a1aa" id="s-restarts">—</span>
      </div>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="color:#71717a;font-size:13px">Jogadores online</span>
        <span style="font-family:monospace;font-size:13px;color:#fbbf24;font-weight:500" id="s-players">—</span>
      </div>
    </div>
  </div>
  <div class="card">
    <div class="section-label">Conexão Direta</div>
    <div style="background:rgba(9,9,11,.6);border:1px solid rgba(251,191,36,.15);border-radius:10px;padding:12px 16px;font-family:monospace;font-size:16px;color:#fbbf24;margin-bottom:10px;word-break:break-all;font-weight:500">{{.ServerIP}}:{{.Server.GamePort}}</div>
    <p style="font-size:11px;color:#52525b;line-height:1.5">Use <em>Direct Connect</em> no menu multiplayer do V Rising</p>
  </div>
</div>

<div class="card">
  <div class="section-label">Configurações</div>
  <div style="display:flex;gap:2px;flex-wrap:wrap;border-bottom:1px solid rgba(63,63,70,.4);margin-bottom:24px;padding-bottom:0">
    {{range $i,$s := .Sections}}
    <button type="button" onclick="setTab('{{$s.ID}}')" data-tab="{{$s.ID}}"
      style="padding:8px 14px;font-size:12px;font-weight:500;background:none;border:none;border-bottom:2px solid transparent;margin-bottom:-1px;cursor:pointer;color:#52525b;transition:all .2s;border-radius:0;white-space:nowrap">{{$s.Label}}</button>
    {{end}}
  </div>
  <form id="cfg-form">
    {{range $i,$s := .Sections}}
    <div class="tab-pane" id="tab-{{$s.ID}}"{{if $i}} style="display:none"{{end}}>
      <div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(260px,1fr));gap:14px">
        {{range $s.Fields}}
        <div>
          <label>{{.Label}}</label>
          {{if eq .Type "select"}}
          <select name="{{.Key}}">
            {{$cur := index $.Config .Key}}
            {{range .Options}}<option value="{{.Value}}"{{if eq .Value $cur}} selected{{end}}>{{.Label}}</option>
            {{end}}
          </select>
          {{else}}
          <input type="{{.Type}}" name="{{.Key}}" value="{{index $.Config .Key}}" placeholder="{{.Placeholder}}">
          {{end}}
        </div>
        {{end}}
      </div>
    </div>
    {{end}}
    <div style="display:flex;justify-content:flex-end;margin-top:24px;padding-top:20px;border-top:1px solid rgba(63,63,70,.4)">
      <button class="btn btn-amber" type="submit" id="save-btn">Salvar e Reiniciar</button>
    </div>
  </form>
</div>

<script>
const serverId='{{.Server.ID}}';
function setTab(id){
  document.querySelectorAll('[data-tab]').forEach(b=>{
    const a=b.dataset.tab===id;
    b.style.color=a?'#fbbf24':'#52525b';
    b.style.borderBottomColor=a?'#fbbf24':'transparent';
  });
  document.querySelectorAll('.tab-pane').forEach(p=>{p.style.display=p.id==='tab-'+id?'':'none'});
}
{{if .Sections}}setTab('{{(index .Sections 0).ID}}');{{end}}
async function refreshStatus(){
  try{
    const r=await fetch('/api/servers/'+serverId+'/status'),s=await r.json();
    const b=document.getElementById('status-badge');
    b.className='badge '+(s.running?'badge-on':'badge-off');
    b.innerHTML='<span class="dot"></span>'+(s.running?'Online':(s.state||'Offline'));
    document.getElementById('s-uptime').textContent=s.uptime||'—';
    document.getElementById('s-state').textContent=s.state||'—';
    document.getElementById('s-restarts').textContent=s.restart_count??'—';
    if(s.max_players){
      const t=s.players+'/'+s.max_players;
      document.getElementById('s-players').textContent=t;
      document.getElementById('players-text').textContent=t+' jogadores';
    }else{document.getElementById('s-players').textContent='—';document.getElementById('players-text').textContent='';}
  }catch(e){}
}
document.getElementById('cfg-form').addEventListener('submit',async e=>{
  e.preventDefault();
  const btn=document.getElementById('save-btn');
  btn.disabled=true;btn.textContent='Salvando...';
  const body={};new FormData(e.target).forEach((v,k)=>body[k]=v);
  try{
    const r=await fetch('/api/servers/'+serverId+'/config',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
    const d=await r.json();
    toast(r.ok?'✓ Configurações salvas, reiniciando...':(d.error||'Erro'),r.ok?'ok':'err');
  }catch(e){toast('Erro de conexão','err');}
  setTimeout(()=>{btn.disabled=false;btn.textContent='Salvar e Reiniciar'},4000);
});
async function restartServer(){
  const btn=document.getElementById('restart-btn');
  btn.disabled=true;btn.textContent='Reiniciando...';
  try{
    const r=await fetch('/api/servers/'+serverId+'/restart',{method:'POST'}),d=await r.json();
    toast(r.ok?'↺ Servidor reiniciando!':(d.error||'Erro'),r.ok?'ok':'err');
  }catch(e){toast('Erro de conexão','err');}
  setTimeout(()=>{btn.disabled=false;btn.textContent='↺ Reiniciar'},6000);
}
refreshStatus();setInterval(refreshStatus,10000);
</script>
{{end}}`
