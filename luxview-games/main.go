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
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const (
	listenAddr             = ":8080"
	sessionCookie          = "gc_session"
	shellSingleQuote       = "'"
	shellSingleQuoteEscape = `'\''`
	bytesPerKiB            = 1024
	percentMultiplier      = 100
	nanoCPUsPerCPU         = 1_000_000_000
	unlimitedResourceLabel = "sem limite"
	defaultCompanionName   = "luxview-games"
	defaultRegistryFile    = "/vrising-data/luxview-games-servers.json"
	legacyRegistryFile     = "/vrising-data/games-companion-servers.json"
	defaultDockerNetwork   = "game-net"
	defaultCustomDataPath  = "/data"
	platformReservedCPU    = 1.0
	platformReservedMemory = 2 * 1024 * 1024 * 1024
	defaultAppCPU          = 0.5
	defaultAppMemory       = 512 * 1024 * 1024
	defaultGameCPU         = 1.0
	defaultGameMemory      = 4 * 1024 * 1024 * 1024
	defaultCustomCPU       = 0.5
	defaultCustomMemory    = 512 * 1024 * 1024
	serverLabelManaged     = "luxview.games"
	serverLabelID          = "luxview.games.id"
	serverLabelTemplate    = "luxview.games.template"
	appLabelManaged        = "luxview.managed"
	appLabelName           = "luxview.app"
	vrisingTemplateID      = "vrising"
	customTemplateID       = "custom"
	portMin                = 1
	portMax                = 65535
)

var (
	managerPassword        = envOr("MANAGER_PASSWORD", "admin")
	serverIP               = envOr("SERVER_IP", "")
	companionContainerName = envOr("COMPANION_CONTAINER", defaultCompanionName)
	registryFile           = envOr("GAMES_REGISTRY_FILE", defaultRegistryFile)
	dockerNetwork          = envOr("GAMES_DOCKER_NETWORK", defaultDockerNetwork)
	sessions               = map[string]time.Time{}
	sessionsMu             sync.Mutex
)

const vrisingIconSVG = `<svg viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M14.5 4.5 19 3l-1.5 4.5-4.7 4.7 2.1 2.1 1.4-1.4 4.8 4.8-3.4 3.4-4.8-4.8 1.4-1.4-2.1-2.1-2.1 2.1 1.4 1.4-4.8 4.8-3.4-3.4 4.8-4.8 1.4 1.4 2.1-2.1-4.7-4.7L5.5 3 10 4.5l2 4.9 2.5-4.9Z" fill="currentColor"/></svg>`
const customGameIconSVG = `<svg viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M4 5.5A2.5 2.5 0 0 1 6.5 3h11A2.5 2.5 0 0 1 20 5.5v13a2.5 2.5 0 0 1-2.5 2.5h-11A2.5 2.5 0 0 1 4 18.5v-13Zm4 4.25H6.75v1.5H8v1.25h1.5v-1.25h1.25v-1.5H9.5V8.5H8v1.25Zm7.5 1.5a1.25 1.25 0 1 0 0-2.5 1.25 1.25 0 0 0 0 2.5Zm2.5 3a1.25 1.25 0 1 0 0-2.5 1.25 1.25 0 0 0 0 2.5ZM7 16.5h10v-1.5H7v1.5Z" fill="currentColor"/></svg>`

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
	IconSVG       template.HTML
	ContainerName string
	GamePort      string
	QueryPort     string
	DataDir       string
	TemplateID    string
	Protocol      string
	Image         string
	Managed       bool
	ConfigFields  []ConfigField
}

type GameTemplate struct {
	ID               string
	DisplayName      string
	Description      string
	IconSVG          template.HTML
	Protocol         string
	DefaultGamePort  string
	DefaultQueryPort string
	DefaultImage     string
	ConfigFields     []ConfigField
	SupportsQuery    bool
	SupportsConfig   bool
}

type ServerRecord struct {
	ID            string `json:"id"`
	TemplateID    string `json:"template_id"`
	DisplayName   string `json:"display_name"`
	ContainerName string `json:"container_name"`
	GamePort      string `json:"game_port"`
	QueryPort     string `json:"query_port,omitempty"`
	Protocol      string `json:"protocol"`
	Image         string `json:"image"`
	CreatedAt     string `json:"created_at"`
}

type ServerRegistry struct {
	Servers []ServerRecord `json:"servers"`
}

type CreateServerRequest struct {
	TemplateID  string `json:"template_id"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Image       string `json:"image"`
	GamePort    string `json:"game_port"`
	QueryPort   string `json:"query_port"`
	CPU         string `json:"cpu"`
	Memory      string `json:"memory"`
	MaxPlayers  string `json:"max_players"`
	Protocol    string `json:"protocol"`
	DataPath    string `json:"data_path"`
	Env         string `json:"env"`
}

type InfraStatus struct {
	HostCPU                 int     `json:"host_cpu"`
	HostMemoryBytes         int64   `json:"host_memory_bytes"`
	HostMemory              string  `json:"host_memory"`
	PlatformReservedCPU     float64 `json:"platform_reserved_cpu"`
	PlatformReservedMemory  int64   `json:"platform_reserved_memory_bytes"`
	PlatformReservedMemoryS string  `json:"platform_reserved_memory"`
	AppsAllocatedCPU        float64 `json:"apps_allocated_cpu"`
	AppsAllocatedMemory     int64   `json:"apps_allocated_memory_bytes"`
	AppsAllocatedMemoryS    string  `json:"apps_allocated_memory"`
	GamesAllocatedCPU       float64 `json:"games_allocated_cpu"`
	GamesAllocatedMemory    int64   `json:"games_allocated_memory_bytes"`
	GamesAllocatedMemoryS   string  `json:"games_allocated_memory"`
	UnboundedGameCPU        float64 `json:"unbounded_game_cpu"`
	UnboundedGameMemory     int64   `json:"unbounded_game_memory_bytes"`
	UnboundedGameMemoryS    string  `json:"unbounded_game_memory"`
	FreeCPU                 float64 `json:"free_cpu"`
	FreeMemoryBytes         int64   `json:"free_memory_bytes"`
	FreeMemory              string  `json:"free_memory"`
	UsedCPU                 float64 `json:"used_cpu"`
	UsedCPUPercent          float64 `json:"used_cpu_percent"`
	UsedMemoryBytes         uint64  `json:"used_memory_bytes"`
	UsedMemory              string  `json:"used_memory"`
	AppsCounted             int     `json:"apps_counted"`
	GamesCounted            int     `json:"games_counted"`
	HasUnboundedGames       bool    `json:"has_unbounded_games"`
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
		IconSVG:       template.HTML(vrisingIconSVG),
		ContainerName: envOr("VRISING_CONTAINER", "luxview-vrising"),
		GamePort:      envOr("VRISING_GAME_PORT", "27015"),
		QueryPort:     envOr("VRISING_QUERY_PORT", "27016"),
		DataDir:       envOr("VRISING_DATA_DIR", "/vrising-data"),
		TemplateID:    vrisingTemplateID,
		Protocol:      "udp",
		Managed:       false,
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

var gameTemplates = []GameTemplate{
	{
		ID:               vrisingTemplateID,
		DisplayName:      "V Rising Dedicated Server",
		Description:      "Nova instância V Rising com portas, volumes e save próprios.",
		IconSVG:          template.HTML(vrisingIconSVG),
		Protocol:         "udp",
		DefaultGamePort:  envOr("VRISING_GAME_PORT", "27015"),
		DefaultQueryPort: envOr("VRISING_QUERY_PORT", "27016"),
		ConfigFields:     gameServers[0].ConfigFields,
		SupportsQuery:    true,
		SupportsConfig:   false,
	},
	{
		ID:              customTemplateID,
		DisplayName:     "Custom Docker Game",
		Description:     "Servidor de outro jogo via imagem Docker, porta publicada e variáveis de ambiente.",
		IconSVG:         template.HTML(customGameIconSVG),
		Protocol:        "udp",
		DefaultGamePort: "27015",
		DefaultImage:    "",
		SupportsQuery:   false,
		SupportsConfig:  false,
	},
}

func allServers() []GameServer {
	servers := make([]GameServer, 0, len(gameServers))
	servers = append(servers, gameServers...)
	for _, record := range loadRegistry().Servers {
		if gs := gameServerFromRecord(record); gs != nil {
			servers = append(servers, *gs)
		}
	}
	return servers
}

func findServer(id string) *GameServer {
	servers := allServers()
	for i := range servers {
		if servers[i].ID == id {
			return &servers[i]
		}
	}
	return nil
}

func findTemplate(id string) *GameTemplate {
	for i := range gameTemplates {
		if gameTemplates[i].ID == id {
			return &gameTemplates[i]
		}
	}
	return nil
}

func gameServerFromRecord(record ServerRecord) *GameServer {
	tmpl := findTemplate(record.TemplateID)
	if tmpl == nil {
		return nil
	}
	return &GameServer{
		ID:            record.ID,
		DisplayName:   record.DisplayName,
		IconSVG:       tmpl.IconSVG,
		ContainerName: record.ContainerName,
		GamePort:      record.GamePort,
		QueryPort:     record.QueryPort,
		TemplateID:    record.TemplateID,
		Protocol:      record.Protocol,
		Image:         record.Image,
		Managed:       true,
		ConfigFields:  managedConfigFields(tmpl),
	}
}

func managedConfigFields(tmpl *GameTemplate) []ConfigField {
	if tmpl.SupportsConfig {
		return tmpl.ConfigFields
	}
	return nil
}

func loadRegistry() ServerRegistry {
	return loadRegistryFromPaths(registryFile, legacyRegistryFile)
}

func loadRegistryFromPaths(primaryPath string, fallbackPath string) ServerRegistry {
	registry, loaded := readRegistry(primaryPath)
	if loaded {
		return registry
	}
	if primaryPath != fallbackPath {
		registry, loaded = readRegistry(fallbackPath)
		if loaded {
			return registry
		}
	}
	return ServerRegistry{}
}

func readRegistry(path string) (ServerRegistry, bool) {
	file, err := os.Open(path)
	if err != nil {
		return ServerRegistry{}, false
	}
	defer file.Close()
	var registry ServerRegistry
	if err := json.NewDecoder(file).Decode(&registry); err != nil {
		log.Println("registry decode error:", err)
		return ServerRegistry{}, true
	}
	return registry, true
}

func saveRegistry(registry ServerRegistry) error {
	if err := os.MkdirAll(filepath.Dir(registryFile), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(registryFile, append(data, '\n'), 0644)
}

func appendServerRecord(record ServerRecord) error {
	registry := loadRegistry()
	for _, existing := range registry.Servers {
		if existing.ID == record.ID {
			return fmt.Errorf("server id already exists")
		}
		if existing.ContainerName == record.ContainerName {
			return fmt.Errorf("container name already exists")
		}
	}
	registry.Servers = append(registry.Servers, record)
	return saveRegistry(registry)
}

func normalizeServerID(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		ok := r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func parsePort(value string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port < portMin || port > portMax {
		return 0, fmt.Errorf("invalid port %q", value)
	}
	return port, nil
}

func protocolOrDefault(protocol string, fallback string) string {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if protocol == "tcp" || protocol == "udp" {
		return protocol
	}
	return fallback
}

func parseEnvLines(raw string) ([]string, error) {
	var env []string
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		key, _, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("invalid env line %q", line)
		}
		env = append(env, line)
	}
	return env, scanner.Err()
}

func serverIDsInUse() map[string]bool {
	used := map[string]bool{}
	for _, server := range allServers() {
		used[server.ID] = true
	}
	return used
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
		buf = buf[:cap(buf)]
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
		cfg[strings.TrimSpace(k)] = parseShellValue(strings.TrimSpace(v))
	}
	return cfg
}

func parseShellValue(v string) string {
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return strings.ReplaceAll(v[1:len(v)-1], shellSingleQuoteEscape, shellSingleQuote)
	}
	return v
}

func shellQuote(v string) string {
	return shellSingleQuote + strings.ReplaceAll(v, shellSingleQuote, shellSingleQuoteEscape) + shellSingleQuote
}

func saveConfig(gs *GameServer, values map[string]string) error {
	var sb strings.Builder
	for _, f := range gs.ConfigFields {
		v := values[f.Key]
		// skip empty numeric/select fields so the entrypoint uses its own defaults;
		// always write text/password fields (even empty, e.g. clearing a password)
		if v == "" && (f.Type == "number" || f.Type == "select") {
			continue
		}
		fmt.Fprintf(&sb, "%s=%s\n", f.Key, shellQuote(v))
	}
	return os.WriteFile(configFilePath(gs), []byte(sb.String()), 0644)
}

// --- Docker ---

type Status struct {
	Running                bool           `json:"running"`
	State                  string         `json:"state"`
	Uptime                 string         `json:"uptime,omitempty"`
	RestartCount           int            `json:"restart_count"`
	Players                int            `json:"players"`
	MaxPlayers             int            `json:"max_players"`
	PlayerQueryError       string         `json:"player_query_error,omitempty"`
	Resources              *ResourceUsage `json:"resources,omitempty"`
	ResourceError          string         `json:"resource_error,omitempty"`
	CompanionResources     *ResourceUsage `json:"companion_resources,omitempty"`
	CompanionResourceError string         `json:"companion_resource_error,omitempty"`
}

type ResourceUsage struct {
	CPUPercent       float64 `json:"cpu_percent"`
	CPUCores         float64 `json:"cpu_cores"`
	CPUUsage         string  `json:"cpu_usage"`
	CPULimitCores    float64 `json:"cpu_limit_cores,omitempty"`
	CPULimit         string  `json:"cpu_limit"`
	MemoryUsageBytes uint64  `json:"memory_usage_bytes"`
	MemoryLimitBytes uint64  `json:"memory_limit_bytes"`
	MemoryUsage      string  `json:"memory_usage"`
	MemoryLimit      string  `json:"memory_limit"`
	NetworkRxBytes   uint64  `json:"network_rx_bytes"`
	NetworkTxBytes   uint64  `json:"network_tx_bytes"`
	NetworkRx        string  `json:"network_rx"`
	NetworkTx        string  `json:"network_tx"`
	PIDs             uint64  `json:"pids"`
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
			queryAddresses := []string{
				gs.ContainerName + ":" + gs.QueryPort,
				"host.docker.internal:" + gs.QueryPort,
			}
			for _, addr := range queryAddresses {
				a2s, err := queryA2S(addr)
				if err == nil {
					s.Players = a2s.Players
					s.MaxPlayers = a2s.MaxPlayers
					s.PlayerQueryError = ""
					break
				}
				s.PlayerQueryError = err.Error()
			}
		}
	}
	resources, err := getContainerResources(ctx, cli, gs.ContainerName)
	if err != nil {
		s.ResourceError = err.Error()
	} else {
		s.Resources = resources
	}
	companionResources, err := getContainerResources(ctx, cli, companionContainerName)
	if err != nil {
		s.CompanionResourceError = err.Error()
	} else {
		s.CompanionResources = companionResources
	}
	return s
}

func getContainerResources(ctx context.Context, cli *dockerclient.Client, name string) (*ResourceUsage, error) {
	info, err := cli.ContainerInspect(ctx, name)
	if err != nil {
		return nil, err
	}
	stats, err := cli.ContainerStats(ctx, name, false)
	if err != nil {
		return nil, err
	}
	defer stats.Body.Close()

	body, err := io.ReadAll(stats.Body)
	if err != nil {
		return nil, err
	}
	var data container.StatsResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	networkRx, networkTx := sumNetworkBytes(data.Networks)
	memoryUsage := memoryUsageBytes(data.MemoryStats.Usage, data.MemoryStats.Stats)
	cpuPercent := calculateCPUPercent(data)
	cpuCores := cpuPercent / percentMultiplier
	var cpuLimit float64
	var memoryLimit uint64
	if info.HostConfig != nil {
		cpuLimit = cpuLimitCores(info.HostConfig.Resources)
		if info.HostConfig.Memory > 0 {
			memoryLimit = uint64(info.HostConfig.Memory)
		}
	}
	return &ResourceUsage{
		CPUPercent:       cpuPercent,
		CPUCores:         cpuCores,
		CPUUsage:         formatCPUCores(cpuCores),
		CPULimitCores:    cpuLimit,
		CPULimit:         formatCPULimit(cpuLimit),
		MemoryUsageBytes: memoryUsage,
		MemoryLimitBytes: memoryLimit,
		MemoryUsage:      formatBytes(memoryUsage),
		MemoryLimit:      formatMemoryLimit(memoryLimit),
		NetworkRxBytes:   networkRx,
		NetworkTxBytes:   networkTx,
		NetworkRx:        formatBytes(networkRx),
		NetworkTx:        formatBytes(networkTx),
		PIDs:             data.PidsStats.Current,
	}, nil
}

func calculateCPUPercent(stats container.StatsResponse) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	if cpuDelta <= 0 || systemDelta <= 0 {
		return 0
	}
	onlineCPUs := float64(stats.CPUStats.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	if onlineCPUs == 0 {
		onlineCPUs = 1
	}
	return (cpuDelta / systemDelta) * onlineCPUs * percentMultiplier
}

func cpuLimitCores(resources container.Resources) float64 {
	if resources.NanoCPUs > 0 {
		return float64(resources.NanoCPUs) / nanoCPUsPerCPU
	}
	if resources.CPUQuota > 0 && resources.CPUPeriod > 0 {
		return float64(resources.CPUQuota) / float64(resources.CPUPeriod)
	}
	return 0
}

func memoryUsageBytes(usage uint64, stats map[string]uint64) uint64 {
	if inactiveFile, ok := stats["inactive_file"]; ok && usage > inactiveFile {
		return usage - inactiveFile
	}
	if cache, ok := stats["cache"]; ok && usage > cache {
		return usage - cache
	}
	return usage
}

func sumNetworkBytes(networks map[string]container.NetworkStats) (uint64, uint64) {
	var rx, tx uint64
	for _, network := range networks {
		rx += network.RxBytes
		tx += network.TxBytes
	}
	return rx, tx
}

func formatCPUCores(cores float64) string {
	return fmt.Sprintf("%.2f cores", cores)
}

func formatCPULimit(cores float64) string {
	if cores <= 0 {
		return unlimitedResourceLabel
	}
	return formatCPUCores(cores)
}

func formatMemoryLimit(bytes uint64) string {
	if bytes == 0 {
		return unlimitedResourceLabel
	}
	return formatBytes(bytes)
}

func formatBytes(bytes uint64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	value := float64(bytes)
	unit := 0
	for value >= bytesPerKiB && unit < len(units)-1 {
		value /= bytesPerKiB
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d %s", bytes, units[unit])
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}

func getInfraStatus(ctx context.Context, cli *dockerclient.Client) (*InfraStatus, error) {
	info, err := cli.Info(ctx)
	if err != nil {
		return nil, err
	}
	status := &InfraStatus{
		HostCPU:                 info.NCPU,
		HostMemoryBytes:         info.MemTotal,
		HostMemory:              formatBytes(uint64(info.MemTotal)),
		PlatformReservedCPU:     platformReservedCPU,
		PlatformReservedMemory:  platformReservedMemory,
		PlatformReservedMemoryS: formatBytes(uint64(platformReservedMemory)),
	}
	appCPU, appMem, appCount := allocatedAppResources(ctx, cli)
	gameCPU, gameMem, unboundedCPU, unboundedMem, gameCount := allocatedGameResources(ctx, cli)
	usedCPU, usedMemory := liveContainerUsage(ctx, cli)
	status.AppsAllocatedCPU = appCPU
	status.AppsAllocatedMemory = appMem
	status.AppsAllocatedMemoryS = formatBytes(uint64(appMem))
	status.GamesAllocatedCPU = gameCPU
	status.GamesAllocatedMemory = gameMem
	status.GamesAllocatedMemoryS = formatBytes(uint64(gameMem))
	status.UnboundedGameCPU = unboundedCPU
	status.UnboundedGameMemory = unboundedMem
	status.UnboundedGameMemoryS = formatBytes(uint64(unboundedMem))
	status.UsedCPU = usedCPU
	status.UsedCPUPercent = cpuPercentOfHost(usedCPU, info.NCPU)
	status.UsedMemoryBytes = usedMemory
	status.UsedMemory = formatBytes(usedMemory)
	status.AppsCounted = appCount
	status.GamesCounted = gameCount
	status.HasUnboundedGames = unboundedCPU > 0 || unboundedMem > 0
	status.FreeCPU = float64(info.NCPU) - platformReservedCPU - appCPU - gameCPU - unboundedCPU
	status.FreeMemoryBytes = info.MemTotal - platformReservedMemory - appMem - gameMem - unboundedMem
	if status.FreeCPU < 0 {
		status.FreeCPU = 0
	}
	if status.FreeMemoryBytes < 0 {
		status.FreeMemoryBytes = 0
	}
	status.FreeMemory = formatBytes(uint64(status.FreeMemoryBytes))
	return status, nil
}

func allocatedAppResources(ctx context.Context, cli *dockerclient.Client) (float64, int64, int) {
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return 0, 0, 0
	}
	var cpu float64
	var memory int64
	var count int
	for _, c := range containers {
		if c.Labels[appLabelManaged] != "true" || c.Labels[appLabelName] == "" {
			continue
		}
		count++
		info, err := cli.ContainerInspect(ctx, c.ID)
		if err != nil || info.HostConfig == nil {
			cpu += defaultAppCPU
			memory += defaultAppMemory
			continue
		}
		if limit := cpuLimitCores(info.HostConfig.Resources); limit > 0 {
			cpu += limit
		} else {
			cpu += defaultAppCPU
		}
		if info.HostConfig.Memory > 0 {
			memory += info.HostConfig.Memory
		} else {
			memory += defaultAppMemory
		}
	}
	return cpu, memory, count
}

func allocatedGameResources(ctx context.Context, cli *dockerclient.Client) (float64, int64, float64, int64, int) {
	var allocatedCPU float64
	var allocatedMemory int64
	var unboundedCPU float64
	var unboundedMemory int64
	servers := allServers()
	for _, server := range servers {
		resources, err := getContainerResources(ctx, cli, server.ContainerName)
		if err != nil {
			continue
		}
		if resources.CPULimitCores > 0 {
			allocatedCPU += resources.CPULimitCores
		} else {
			unboundedCPU += resources.CPUCores
		}
		if resources.MemoryLimitBytes > 0 {
			allocatedMemory += int64(resources.MemoryLimitBytes)
		} else {
			unboundedMemory += int64(resources.MemoryUsageBytes)
		}
	}
	return allocatedCPU, allocatedMemory, unboundedCPU, unboundedMemory, len(servers)
}

func liveContainerUsage(ctx context.Context, cli *dockerclient.Client) (float64, uint64) {
	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return 0, 0
	}
	statsCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var cpu float64
	var memory uint64
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, c := range containers {
		wg.Add(1)
		go func(containerID string) {
			defer wg.Done()
			resources, err := getContainerResources(statsCtx, cli, containerID)
			if err != nil {
				return
			}
			mu.Lock()
			cpu += resources.CPUCores
			memory += resources.MemoryUsageBytes
			mu.Unlock()
		}(c.ID)
	}
	wg.Wait()
	return cpu, memory
}

func cpuPercentOfHost(cores float64, hostCPU int) float64 {
	if hostCPU <= 0 {
		return 0
	}
	return cores / float64(hostCPU) * percentMultiplier
}

func parseCPUOrDefault(raw string, fallback float64) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	cpu, err := strconv.ParseFloat(raw, 64)
	if err != nil || cpu <= 0 {
		return 0, fmt.Errorf("CPU inválida")
	}
	return cpu, nil
}

func parseMemoryLimitOrDefault(raw string, fallback int64) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	memory := parseMemoryString(raw)
	if memory <= 0 {
		return 0, fmt.Errorf("memória inválida")
	}
	return memory, nil
}

func parseMemoryString(s string) int64 {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0
	}
	suffix := s[len(s)-1]
	numStr := s[:len(s)-1]
	val, err := strconv.ParseFloat(numStr, 64)
	if err == nil {
		switch suffix {
		case 'g':
			return int64(val * bytesPerKiB * bytesPerKiB * bytesPerKiB)
		case 'm':
			return int64(val * bytesPerKiB * bytesPerKiB)
		case 'k':
			return int64(val * bytesPerKiB)
		}
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func validateInfraCapacity(ctx context.Context, cli *dockerclient.Client, cpu float64, memory int64) error {
	status, err := getInfraStatus(ctx, cli)
	if err != nil {
		return err
	}
	if cpu > status.FreeCPU {
		return fmt.Errorf("CPU excede capacidade disponível: livre %.2f cores, solicitado %.2f cores", status.FreeCPU, cpu)
	}
	if memory > status.FreeMemoryBytes {
		return fmt.Errorf("memória excede capacidade disponível: livre %s, solicitado %s", status.FreeMemory, formatBytes(uint64(memory)))
	}
	return nil
}

func createManagedServer(ctx context.Context, cli *dockerclient.Client, req CreateServerRequest) (*GameServer, error) {
	tmpl := findTemplate(req.TemplateID)
	if tmpl == nil {
		return nil, fmt.Errorf("template inválido")
	}
	id := normalizeServerID(req.ID)
	if id == "" {
		id = normalizeServerID(req.DisplayName)
	}
	if id == "" {
		return nil, fmt.Errorf("id inválido")
	}
	if serverIDsInUse()[id] {
		return nil, fmt.Errorf("já existe um servidor com esse id")
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = id
	}
	gamePort := strings.TrimSpace(req.GamePort)
	if gamePort == "" {
		gamePort = tmpl.DefaultGamePort
	}
	if _, err := parsePort(gamePort); err != nil {
		return nil, err
	}
	queryPort := strings.TrimSpace(req.QueryPort)
	if tmpl.SupportsQuery {
		if queryPort == "" {
			queryPort = nextPort(gamePort)
		}
		if _, err := parsePort(queryPort); err != nil {
			return nil, err
		}
	}
	protocol := protocolOrDefault(req.Protocol, tmpl.Protocol)
	containerName := managedContainerName(tmpl.ID, id)
	defaultCPU := defaultGameCPU
	defaultMemory := int64(defaultGameMemory)
	if req.TemplateID == customTemplateID {
		defaultCPU = defaultCustomCPU
		defaultMemory = defaultCustomMemory
	}
	cpuLimit, err := parseCPUOrDefault(req.CPU, defaultCPU)
	if err != nil {
		return nil, err
	}
	memoryLimit, err := parseMemoryLimitOrDefault(req.Memory, defaultMemory)
	if err != nil {
		return nil, err
	}
	if err := validateInfraCapacity(ctx, cli, cpuLimit, memoryLimit); err != nil {
		return nil, err
	}
	if req.TemplateID == vrisingTemplateID {
		return createVRisingServer(ctx, cli, tmpl, id, displayName, containerName, gamePort, queryPort, req.MaxPlayers, cpuLimit, memoryLimit)
	}
	return createCustomServer(ctx, cli, tmpl, id, displayName, containerName, gamePort, protocol, req, cpuLimit, memoryLimit)
}

func nextPort(port string) string {
	value, err := parsePort(port)
	if err != nil || value >= portMax {
		return ""
	}
	return strconv.Itoa(value + 1)
}

func managedContainerName(templateID string, id string) string {
	if templateID == vrisingTemplateID {
		return "luxview-vrising-" + id
	}
	return "luxview-game-" + id
}

func createVRisingServer(ctx context.Context, cli *dockerclient.Client, tmpl *GameTemplate, id string, displayName string, containerName string, gamePort string, queryPort string, maxPlayers string, cpuLimit float64, memoryLimit int64) (*GameServer, error) {
	imageName := envOr("VRISING_IMAGE", "")
	if imageName == "" {
		if info, err := cli.ContainerInspect(ctx, gameServers[0].ContainerName); err == nil && info.Config != nil {
			imageName = info.Config.Image
		}
	}
	if imageName == "" {
		imageName = "luxview-cloud-vrising:latest"
	}
	if maxPlayers = strings.TrimSpace(maxPlayers); maxPlayers == "" {
		maxPlayers = envOr("VRISING_MAX_USERS", "40")
	}
	serverVolume := "luxview-vrising-" + id + "-server"
	dataVolume := "luxview-vrising-" + id + "-data"
	env := []string{
		"VRISING_SERVER_NAME=" + displayName,
		"VRISING_SAVE_NAME=world1",
		"VRISING_GAME_PORT=" + gamePort,
		"VRISING_QUERY_PORT=" + queryPort,
		"VRISING_MAX_USERS=" + maxPlayers,
		"VRISING_PRESET=" + envOr("VRISING_PRESET", "StandardPvP"),
	}
	exposedPorts, portBindings, err := portConfig([]publishedPort{
		{ContainerPort: gamePort, HostPort: gamePort, Protocol: "udp"},
		{ContainerPort: queryPort, HostPort: queryPort, Protocol: "udp"},
	})
	if err != nil {
		return nil, err
	}
	if err := createContainer(ctx, cli, containerCreateSpec{
		Name:         containerName,
		Image:        imageName,
		Env:          env,
		ExposedPorts: exposedPorts,
		PortBindings: portBindings,
		Mounts: []mount.Mount{
			{Type: mount.TypeVolume, Source: serverVolume, Target: "/vrising-server"},
			{Type: mount.TypeVolume, Source: dataVolume, Target: "/vrising-data"},
		},
		Labels:      serverLabels(id, tmpl.ID),
		StopTimeout: 60,
		CPULimit:    cpuLimit,
		MemoryLimit: memoryLimit,
	}); err != nil {
		return nil, err
	}
	record := ServerRecord{
		ID:            id,
		TemplateID:    tmpl.ID,
		DisplayName:   displayName,
		ContainerName: containerName,
		GamePort:      gamePort,
		QueryPort:     queryPort,
		Protocol:      "udp",
		Image:         imageName,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	if err := appendServerRecord(record); err != nil {
		return nil, err
	}
	return gameServerFromRecord(record), nil
}

func createCustomServer(ctx context.Context, cli *dockerclient.Client, tmpl *GameTemplate, id string, displayName string, containerName string, gamePort string, protocol string, req CreateServerRequest, cpuLimit float64, memoryLimit int64) (*GameServer, error) {
	imageName := strings.TrimSpace(req.Image)
	if imageName == "" {
		return nil, fmt.Errorf("imagem Docker obrigatória")
	}
	env, err := parseEnvLines(req.Env)
	if err != nil {
		return nil, err
	}
	dataPath := strings.TrimSpace(req.DataPath)
	if dataPath == "" {
		dataPath = defaultCustomDataPath
	}
	exposedPorts, portBindings, err := portConfig([]publishedPort{{ContainerPort: gamePort, HostPort: gamePort, Protocol: protocol}})
	if err != nil {
		return nil, err
	}
	if err := pullImage(ctx, cli, imageName); err != nil {
		return nil, err
	}
	if err := createContainer(ctx, cli, containerCreateSpec{
		Name:         containerName,
		Image:        imageName,
		Env:          env,
		ExposedPorts: exposedPorts,
		PortBindings: portBindings,
		Mounts:       []mount.Mount{{Type: mount.TypeVolume, Source: "luxview-game-" + id + "-data", Target: dataPath}},
		Labels:       serverLabels(id, tmpl.ID),
		StopTimeout:  30,
		CPULimit:     cpuLimit,
		MemoryLimit:  memoryLimit,
	}); err != nil {
		return nil, err
	}
	record := ServerRecord{
		ID:            id,
		TemplateID:    tmpl.ID,
		DisplayName:   displayName,
		ContainerName: containerName,
		GamePort:      gamePort,
		Protocol:      protocol,
		Image:         imageName,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	if err := appendServerRecord(record); err != nil {
		return nil, err
	}
	return gameServerFromRecord(record), nil
}

type publishedPort struct {
	ContainerPort string
	HostPort      string
	Protocol      string
}

func portConfig(ports []publishedPort) (nat.PortSet, nat.PortMap, error) {
	exposedPorts := nat.PortSet{}
	portBindings := nat.PortMap{}
	for _, p := range ports {
		if _, err := parsePort(p.ContainerPort); err != nil {
			return nil, nil, err
		}
		if _, err := parsePort(p.HostPort); err != nil {
			return nil, nil, err
		}
		protocol := protocolOrDefault(p.Protocol, "udp")
		port := nat.Port(p.ContainerPort + "/" + protocol)
		exposedPorts[port] = struct{}{}
		portBindings[port] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: p.HostPort}}
	}
	return exposedPorts, portBindings, nil
}

type containerCreateSpec struct {
	Name         string
	Image        string
	Env          []string
	ExposedPorts nat.PortSet
	PortBindings nat.PortMap
	Mounts       []mount.Mount
	Labels       map[string]string
	StopTimeout  int
	CPULimit     float64
	MemoryLimit  int64
}

func createContainer(ctx context.Context, cli *dockerclient.Client, spec containerCreateSpec) error {
	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image:        spec.Image,
			Env:          spec.Env,
			ExposedPorts: spec.ExposedPorts,
			Labels:       spec.Labels,
			StopTimeout:  &spec.StopTimeout,
		},
		&container.HostConfig{
			RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
			PortBindings:  spec.PortBindings,
			Mounts:        spec.Mounts,
			Resources: container.Resources{
				NanoCPUs: int64(spec.CPULimit * nanoCPUsPerCPU),
				Memory:   spec.MemoryLimit,
			},
		},
		&network.NetworkingConfig{EndpointsConfig: map[string]*network.EndpointSettings{
			dockerNetwork: {},
		}},
		nil,
		spec.Name,
	)
	if err != nil {
		return err
	}
	return cli.ContainerStart(ctx, resp.ID, container.StartOptions{})
}

func pullImage(ctx context.Context, cli *dockerclient.Client, imageName string) error {
	rc, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = io.Copy(io.Discard, rc)
	return err
}

func serverLabels(id string, templateID string) map[string]string {
	return map[string]string{
		serverLabelManaged:  "true",
		serverLabelID:       id,
		serverLabelTemplate: templateID,
	}
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
	mux.HandleFunc("GET /servers/new", auth(newServerPageHandler))
	mux.HandleFunc("GET /servers/{id}", auth(serverPageHandler))
	mux.HandleFunc("GET /api/infra/status", auth(apiInfraStatusHandler(cli)))
	mux.HandleFunc("POST /api/servers", auth(apiCreateServerHandler(cli)))
	mux.HandleFunc("GET /api/servers/{id}/status", auth(apiStatusHandler(cli)))
	mux.HandleFunc("GET /api/servers/{id}/config", auth(apiGetConfigHandler))
	mux.HandleFunc("POST /api/servers/{id}/config", auth(apiSetConfigHandler(cli)))
	mux.HandleFunc("POST /api/servers/{id}/restart", auth(apiRestartHandler(cli)))

	log.Println("luxview-games listening on", listenAddr)
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
	renderPage(w, indexTmpl, map[string]any{"Servers": allServers()})
}

func newServerPageHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, newServerTmpl, map[string]any{"Templates": gameTemplates})
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

func apiInfraStatusHandler(cli *dockerclient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := getInfraStatus(r.Context(), cli)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}
}

func apiCreateServerHandler(cli *dockerclient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateServerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid body")
			return
		}
		server, err := createManagedServer(r.Context(), cli, req)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": server.ID, "url": "/servers/" + server.ID})
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
			writeJSONError(w, http.StatusBadRequest, "invalid body")
			return
		}
		if err := saveConfig(gs, values); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to save")
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

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
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
<title>Luxview Games</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{background:#09090b;color:#fafafa;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;min-height:100vh;font-size:14px;-webkit-font-smoothing:antialiased}
::-webkit-scrollbar{width:5px;height:5px}
::-webkit-scrollbar-track{background:transparent}
::-webkit-scrollbar-thumb{background:#3f3f46;border-radius:9999px}
a{color:inherit;text-decoration:none}
/* floating pill nav */
.nav{position:fixed;top:20px;left:50%;transform:translateX(-50%);display:flex;align-items:center;gap:16px;height:48px;padding:0 20px;background:rgba(9,9,11,.85);backdrop-filter:blur(16px);-webkit-backdrop-filter:blur(16px);border:1px solid rgba(63,63,70,.5);border-radius:9999px;box-shadow:0 20px 60px rgba(0,0,0,.5);z-index:50;white-space:nowrap}
.nav-logo{width:32px;height:32px;background:linear-gradient(135deg,#fbbf24,#f59e0b);border-radius:8px;display:flex;align-items:center;justify-content:center;color:#4c1d95;flex-shrink:0;box-shadow:0 0 15px rgba(251,191,36,.25)}
.nav-logo svg{width:18px;height:18px;display:block}
.nav-title{font-size:13px;font-weight:600;color:#fafafa}
.nav-sep{width:1px;height:20px;background:rgba(63,63,70,.7)}
.server-icon{width:48px;height:48px;background:rgba(251,191,36,.08);border:1px solid rgba(251,191,36,.15);border-radius:12px;display:flex;align-items:center;justify-content:center;color:#d4d4d8;flex-shrink:0}
.server-icon-lg{width:56px;height:56px;border-radius:14px;color:#d4d4d8}
.server-icon svg{width:26px;height:26px;display:block}
.server-icon-lg svg{width:30px;height:30px}
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
    <div class="nav-logo"><svg viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M7 8h10a5 5 0 0 1 4.8 3.6l.8 2.8a2.8 2.8 0 0 1-4.6 2.7L15.7 15H8.3L6 17.1a2.8 2.8 0 0 1-4.6-2.7l.8-2.8A5 5 0 0 1 7 8Zm1 3.2H6.6v1.4H5.2V14h1.4v1.4H8V14h1.4v-1.4H8v-1.4Zm8.8 1.1a1 1 0 1 0 0-2 1 1 0 0 0 0 2Zm2.2 3a1 1 0 1 0 0-2 1 1 0 0 0 0 2Z" fill="currentColor"/></svg></div>
    <span class="nav-title">Luxview Games</span>
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
      <div style="width:56px;height:56px;background:linear-gradient(135deg,#fbbf24,#f59e0b);border-radius:14px;display:flex;align-items:center;justify-content:center;color:#4c1d95;margin:0 auto 16px;box-shadow:0 0 30px rgba(251,191,36,.25)"><svg viewBox="0 0 24 24" aria-hidden="true" focusable="false" style="width:30px;height:30px;display:block"><path d="M7 8h10a5 5 0 0 1 4.8 3.6l.8 2.8a2.8 2.8 0 0 1-4.6 2.7L15.7 15H8.3L6 17.1a2.8 2.8 0 0 1-4.6-2.7l.8-2.8A5 5 0 0 1 7 8Zm1 3.2H6.6v1.4H5.2V14h1.4v1.4H8V14h1.4v-1.4H8v-1.4Zm8.8 1.1a1 1 0 1 0 0-2 1 1 0 0 0 0 2Zm2.2 3a1 1 0 1 0 0-2 1 1 0 0 0 0 2Z" fill="currentColor"/></svg></div>
      <h1 style="font-size:22px;font-weight:700;margin-bottom:6px">Luxview Games</h1>
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
<div style="display:flex;align-items:flex-start;justify-content:space-between;gap:16px;flex-wrap:wrap;margin-bottom:28px">
  <div>
    <h1 style="font-size:22px;font-weight:700">Servidores</h1>
    <p style="font-size:13px;color:#71717a;margin-top:4px">{{len .Servers}} servidor(es) gerenciado(s)</p>
  </div>
  <a class="btn btn-amber" href="/servers/new">Novo servidor</a>
</div>
<div class="card" style="margin-bottom:20px">
  <div class="section-label">Uso da máquina</div>
  <div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:16px">
    <div>
      <div style="font-size:12px;color:#71717a;margin-bottom:6px">CPU em uso agora</div>
      <div style="font-family:monospace;font-size:18px;color:#fafafa" id="infra-used-cpu">—</div>
      <div style="font-size:11px;color:#52525b;margin-top:4px" id="infra-cpu-total">—</div>
    </div>
    <div>
      <div style="font-size:12px;color:#71717a;margin-bottom:6px">Memória em uso agora</div>
      <div style="font-family:monospace;font-size:18px;color:#fafafa" id="infra-used-memory">—</div>
      <div style="font-size:11px;color:#52525b;margin-top:4px" id="infra-memory-total">—</div>
    </div>
    <div>
      <div style="font-size:12px;color:#71717a;margin-bottom:6px">Alocado apps</div>
      <div style="font-family:monospace;font-size:18px;color:#fafafa" id="infra-apps">—</div>
      <div style="font-size:11px;color:#52525b;margin-top:4px" id="infra-apps-memory">—</div>
    </div>
    <div>
      <div style="font-size:12px;color:#71717a;margin-bottom:6px">Livre para alocar</div>
      <div style="font-family:monospace;font-size:18px;color:#fbbf24" id="infra-free">—</div>
      <div style="font-size:11px;color:#52525b;margin-top:4px" id="infra-free-memory">—</div>
    </div>
  </div>
  <div style="font-size:11px;color:#52525b;margin-top:16px" id="infra-note">Carregando recursos...</div>
</div>
<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(290px,1fr));gap:16px">
  {{range .Servers}}
  <a href="/servers/{{.ID}}">
    <div class="card card-hover" style="cursor:pointer">
      <div style="display:flex;align-items:center;gap:14px;margin-bottom:20px">
        <div class="server-icon">{{.IconSVG}}</div>
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
async function refreshInfra(){
  try{
    const r=await fetch('/api/infra/status'),s=await r.json();
    document.getElementById('infra-used-cpu').textContent=s.used_cpu.toFixed(2)+' cores ('+s.used_cpu_percent.toFixed(1)+'%)';
    document.getElementById('infra-cpu-total').textContent=s.host_cpu+' cores totais · plataforma reserva '+s.platform_reserved_cpu.toFixed(1);
    document.getElementById('infra-used-memory').textContent=s.used_memory+' / '+s.host_memory;
    document.getElementById('infra-memory-total').textContent='plataforma reserva '+s.platform_reserved_memory;
    document.getElementById('infra-apps').textContent=s.apps_allocated_cpu.toFixed(2)+' cores';
    document.getElementById('infra-apps-memory').textContent=s.apps_allocated_memory+' em '+s.apps_counted+' apps';
    document.getElementById('infra-free').textContent=s.free_cpu.toFixed(2)+' cores';
    document.getElementById('infra-free-memory').textContent=s.free_memory+' livres';
    const game='jogos alocados '+s.games_allocated_cpu.toFixed(2)+' cores / '+s.games_allocated_memory;
    const legacy=s.has_unbounded_games?' · há servidor sem limite; livre considera uso atual dele':'';
    document.getElementById('infra-note').textContent=game+legacy;
  }catch(e){document.getElementById('infra-note').textContent='recursos indisponíveis'}
}
refreshInfra();setInterval(refreshInfra,10000);
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

const newServerTmpl = `{{define "content"}}
<div style="margin-bottom:8px">
  <a href="/" style="font-size:13px;color:#52525b;display:inline-flex;align-items:center;gap:4px;transition:color .2s" onmouseover="this.style.color='#a1a1aa'" onmouseout="this.style.color='#52525b'">← Servidores</a>
</div>
<div style="margin-bottom:24px">
  <h1 style="font-size:22px;font-weight:700">Novo servidor</h1>
  <p style="font-size:13px;color:#71717a;margin-top:4px">Crie uma instância V Rising ou um servidor customizado via imagem Docker.</p>
</div>
<div class="card">
  <form id="create-form">
    <div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(260px,1fr));gap:14px">
      <div>
        <label>Tipo</label>
        <select name="template_id" id="template">
          {{range .Templates}}<option value="{{.ID}}" data-game-port="{{.DefaultGamePort}}" data-query-port="{{.DefaultQueryPort}}" data-protocol="{{.Protocol}}" data-custom="{{if eq .ID "custom"}}1{{else}}0{{end}}">{{.DisplayName}}</option>{{end}}
        </select>
      </div>
      <div>
        <label>Nome</label>
        <input type="text" name="display_name" id="display-name" placeholder="Servidor PvP 2">
      </div>
      <div>
        <label>ID</label>
        <input type="text" name="id" id="server-id" placeholder="pvp-2">
      </div>
      <div>
        <label>Porta do jogo</label>
        <input type="number" name="game_port" id="game-port" placeholder="27017">
      </div>
      <div id="query-port-wrap">
        <label>Porta query</label>
        <input type="number" name="query_port" id="query-port" placeholder="27018">
      </div>
      <div>
        <label>CPU reservada</label>
        <input type="number" step="0.1" min="0.1" name="cpu" id="cpu-limit" placeholder="1.0">
      </div>
      <div>
        <label>Memória reservada</label>
        <input type="text" name="memory" id="memory-limit" placeholder="4g">
      </div>
      <div>
        <label>Máx. jogadores</label>
        <input type="number" name="max_players" placeholder="40">
      </div>
      <div id="protocol-wrap" style="display:none">
        <label>Protocolo</label>
        <select name="protocol" id="protocol">
          <option value="udp">UDP</option>
          <option value="tcp">TCP</option>
        </select>
      </div>
      <div id="image-wrap" style="display:none">
        <label>Imagem Docker</label>
        <input type="text" name="image" id="image" placeholder="exemplo/jogo-server:latest">
      </div>
      <div id="data-path-wrap" style="display:none">
        <label>Mount de dados no container</label>
        <input type="text" name="data_path" id="data-path" value="/data">
      </div>
      <div id="env-wrap" style="display:none;grid-column:1/-1">
        <label>Variáveis de ambiente</label>
        <textarea name="env" rows="8" placeholder="KEY=value&#10;ANOTHER_KEY=value" style="width:100%;background:rgba(9,9,11,.7);border:1px solid rgba(63,63,70,.6);border-radius:10px;padding:8px 12px;color:#fafafa;font-size:13px;outline:none;resize:vertical"></textarea>
      </div>
    </div>
    <div style="display:flex;justify-content:flex-end;margin-top:24px;padding-top:20px;border-top:1px solid rgba(63,63,70,.4)">
      <button class="btn btn-amber" type="submit" id="create-btn">Criar servidor</button>
    </div>
  </form>
</div>
<script>
const template=document.getElementById('template');
function syncTemplate(){
  const opt=template.selectedOptions[0];
  const custom=opt.dataset.custom==='1';
  document.getElementById('image-wrap').style.display=custom?'':'none';
  document.getElementById('protocol-wrap').style.display=custom?'':'none';
  document.getElementById('data-path-wrap').style.display=custom?'':'none';
  document.getElementById('env-wrap').style.display=custom?'':'none';
  document.getElementById('query-port-wrap').style.display=custom?'none':'';
  document.getElementById('game-port').placeholder=opt.dataset.gamePort||'27015';
  document.getElementById('query-port').placeholder=opt.dataset.queryPort||'';
  document.getElementById('cpu-limit').placeholder=custom?'0.5':'1.0';
  document.getElementById('memory-limit').placeholder=custom?'512m':'4g';
  document.getElementById('protocol').value=opt.dataset.protocol||'udp';
}
function slug(v){return v.toLowerCase().trim().replace(/[^a-z0-9]+/g,'-').replace(/^-+|-+$/g,'')}
template.addEventListener('change',syncTemplate);
document.getElementById('display-name').addEventListener('input',e=>{
  const id=document.getElementById('server-id');
  if(!id.dataset.touched){id.value=slug(e.target.value)}
});
document.getElementById('server-id').addEventListener('input',e=>{e.target.dataset.touched='1'});
document.getElementById('create-form').addEventListener('submit',async e=>{
  e.preventDefault();
  const btn=document.getElementById('create-btn');
  btn.disabled=true;btn.textContent='Criando...';
  const body={};new FormData(e.target).forEach((v,k)=>body[k]=v);
  try{
    const r=await fetch('/api/servers',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
    const d=await r.json();
    if(r.ok){toast('Servidor criado','ok');setTimeout(()=>location.href=d.url,500)}
    else{toast(d.error||'Erro ao criar servidor','err')}
  }catch(e){toast('Erro de conexão','err')}
  setTimeout(()=>{btn.disabled=false;btn.textContent='Criar servidor'},2500);
});
syncTemplate();
</script>
{{end}}`

const serverTmpl = `{{define "content"}}
<div style="margin-bottom:8px">
  <a href="/" style="font-size:13px;color:#52525b;display:inline-flex;align-items:center;gap:4px;transition:color .2s" onmouseover="this.style.color='#a1a1aa'" onmouseout="this.style.color='#52525b'">← Servidores</a>
</div>
<div style="display:flex;align-items:flex-start;justify-content:space-between;flex-wrap:wrap;gap:16px;margin-bottom:24px">
  <div style="display:flex;align-items:center;gap:16px">
    <div class="server-icon server-icon-lg">{{.Server.IconSVG}}</div>
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

<div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(280px,1fr));gap:16px;margin-bottom:20px">
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
    <p style="font-size:11px;color:#52525b;line-height:1.5">Use o endereço acima no cliente do jogo.</p>
  </div>
  <div class="card">
    <div class="section-label">Recursos do servidor</div>
    <div style="display:flex;flex-direction:column;gap:12px">
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="color:#71717a;font-size:13px">CPU</span>
        <span style="font-family:monospace;font-size:13px;color:#a1a1aa" id="r-cpu">—</span>
      </div>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="color:#71717a;font-size:13px">Limite CPU</span>
        <span style="font-family:monospace;font-size:13px;color:#a1a1aa" id="r-cpu-limit">—</span>
      </div>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="color:#71717a;font-size:13px">Memória</span>
        <span style="font-family:monospace;font-size:13px;color:#a1a1aa" id="r-memory">—</span>
      </div>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="color:#71717a;font-size:13px">Rede ↓</span>
        <span style="font-family:monospace;font-size:13px;color:#a1a1aa" id="r-rx">—</span>
      </div>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="color:#71717a;font-size:13px">Rede ↑</span>
        <span style="font-family:monospace;font-size:13px;color:#a1a1aa" id="r-tx">—</span>
      </div>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="color:#71717a;font-size:13px">Processos</span>
        <span style="font-family:monospace;font-size:13px;color:#a1a1aa" id="r-pids">—</span>
      </div>
    </div>
  </div>
  <div class="card">
    <div class="section-label">Recursos do Companion</div>
    <div style="display:flex;flex-direction:column;gap:12px">
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="color:#71717a;font-size:13px">CPU</span>
        <span style="font-family:monospace;font-size:13px;color:#a1a1aa" id="c-cpu">—</span>
      </div>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="color:#71717a;font-size:13px">Limite CPU</span>
        <span style="font-family:monospace;font-size:13px;color:#a1a1aa" id="c-cpu-limit">—</span>
      </div>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="color:#71717a;font-size:13px">Memória</span>
        <span style="font-family:monospace;font-size:13px;color:#a1a1aa" id="c-memory">—</span>
      </div>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="color:#71717a;font-size:13px">Rede ↓</span>
        <span style="font-family:monospace;font-size:13px;color:#a1a1aa" id="c-rx">—</span>
      </div>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="color:#71717a;font-size:13px">Rede ↑</span>
        <span style="font-family:monospace;font-size:13px;color:#a1a1aa" id="c-tx">—</span>
      </div>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="color:#71717a;font-size:13px">Processos</span>
        <span style="font-family:monospace;font-size:13px;color:#a1a1aa" id="c-pids">—</span>
      </div>
    </div>
  </div>
</div>

{{if .Sections}}
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
{{end}}

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
function setResourceFields(prefix, resources, error){
  const set=(name,value)=>{document.getElementById(prefix+'-'+name).textContent=value};
  if(resources){
    set('cpu',resources.cpu_usage+' ('+resources.cpu_percent.toFixed(1)+'%)');
    set('cpu-limit',resources.cpu_limit);
    set('memory',resources.memory_usage+' / '+resources.memory_limit);
    set('rx',resources.network_rx);
    set('tx',resources.network_tx);
    set('pids',resources.pids||'—');
  }else{
    set('cpu',error?'indisponível':'—');
    set('cpu-limit','—');
    set('memory','—');
    set('rx','—');
    set('tx','—');
    set('pids','—');
  }
}
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
    }else{
      document.getElementById('s-players').textContent=s.player_query_error?'indisponível':'—';
      document.getElementById('players-text').textContent=s.player_query_error?'query indisponível':'';
    }
    setResourceFields('r',s.resources,s.resource_error);
    setResourceFields('c',s.companion_resources,s.companion_resource_error);
  }catch(e){}
}
{{if .Sections}}
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
{{end}}
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
