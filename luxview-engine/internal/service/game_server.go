package service

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	dockerclient "github.com/luxview/engine/pkg/docker"
	"github.com/luxview/engine/pkg/logger"
)

// GameServerService manages game server containers.
type GameServerService struct {
	docker      *dockerclient.Client
	gameNetwork string
	appRepo     *repository.AppRepo
	portManager *PortManager
	serverIP    string // VPS public IP, injected so game entrypoints can advertise it
}

func NewGameServerService(docker *dockerclient.Client, gameNetwork string, appRepo *repository.AppRepo, portManager *PortManager, serverIP string) *GameServerService {
	return &GameServerService{docker: docker, gameNetwork: gameNetwork, appRepo: appRepo, portManager: portManager, serverIP: serverIP}
}

// ContainerName returns the Docker container name for a game server app.
func ContainerName(subdomain string) string {
	return fmt.Sprintf("luxview-game-%s", subdomain)
}

// buildMounts returns the Docker mount list for a game server, preferring the
// multi-volume Volumes field and falling back to the legacy DataVolume/DataDir.
func buildMounts(subdomain string, cfg *model.GameServerConfig) []mount.Mount {
	if len(cfg.Volumes) > 0 {
		mounts := make([]mount.Mount, 0, len(cfg.Volumes))
		for _, v := range cfg.Volumes {
			mounts = append(mounts, mount.Mount{
				Type:   mount.TypeVolume,
				Source: v.Name,
				Target: v.MountPath,
			})
		}
		return mounts
	}
	dataVolume := cfg.DataVolume
	if dataVolume == "" {
		dataVolume = fmt.Sprintf("luxview-game-%s-data", subdomain)
	}
	dataDir := cfg.DataDir
	if dataDir == "" {
		dataDir = "/data"
	}
	return []mount.Mount{{Type: mount.TypeVolume, Source: dataVolume, Target: dataDir}}
}

// Start creates and starts a game server container.
func (s *GameServerService) Start(ctx context.Context, app *model.App, cfg *model.GameServerConfig) (string, error) {
	log := logger.With("game-server")
	containerName := ContainerName(app.Subdomain)

	protocol := cfg.Protocol
	if protocol == "" {
		protocol = "udp"
	}
	gamePortStr := fmt.Sprintf("%d/%s", cfg.GamePort, protocol)
	gamePort := nat.Port(gamePortStr)

	portSet := nat.PortSet{gamePort: struct{}{}}
	portMap := nat.PortMap{
		gamePort: []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: strconv.Itoa(cfg.GamePort)},
		},
	}

	if cfg.QueryPort > 0 {
		queryPortStr := fmt.Sprintf("%d/%s", cfg.QueryPort, protocol)
		queryPort := nat.Port(queryPortStr)
		portSet[queryPort] = struct{}{}
		portMap[queryPort] = []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: strconv.Itoa(cfg.QueryPort)},
		}
	}

	for _, ep := range cfg.ExtraPorts {
		epProto := ep.Protocol
		if epProto == "" {
			epProto = protocol
		}
		epStr := fmt.Sprintf("%d/%s", ep.Port, epProto)
		epNat := nat.Port(epStr)
		portSet[epNat] = struct{}{}
		portMap[epNat] = []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: strconv.Itoa(ep.Port)},
		}
	}

	// HTTP web service (auth/admin panel) routed via Traefik subdomain. The
	// template's WebPort (container side) is published to the app's AssignedPort
	// (host side); router.go then routes "<subdomain>.<domain>" to it in plain
	// HTTP. Allocate AssignedPort on first start if the app doesn't have one.
	if tmpl := GetGameTemplate(cfg.TemplateID); tmpl != nil && tmpl.WebPort > 0 {
		if app.AssignedPort == 0 && s.portManager != nil && s.appRepo != nil {
			port, err := s.portManager.Allocate(ctx)
			if err != nil {
				return "", fmt.Errorf("allocate web port: %w", err)
			}
			if err := s.appRepo.UpdatePort(ctx, app.ID, port); err != nil {
				s.portManager.Release(port)
				return "", fmt.Errorf("persist web port: %w", err)
			}
			app.AssignedPort = port
		}
		if app.AssignedPort > 0 {
			webNat := nat.Port(fmt.Sprintf("%d/tcp", tmpl.WebPort))
			portSet[webNat] = struct{}{}
			portMap[webNat] = []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: strconv.Itoa(app.AssignedPort)},
			}
		}
	}

	var envList []string
	for k, v := range cfg.ConfigFields {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}
	// Public IP so game entrypoints can advertise the server to remote clients
	// (e.g. Rakion's broker GameServers.ini wan=<public ip>).
	if s.serverIP != "" {
		envList = append(envList, "LUXVIEW_PUBLIC_IP="+s.serverIP)
	}

	// Build mount list. Prefer the multi-volume Volumes field; fall back to the legacy
	// single DataVolume/DataDir pair when Volumes is empty (older game configs).
	mounts := buildMounts(app.Subdomain, cfg)

	nanoCPUs, memory := parseResourceLimits(app.ResourceLimits)

	containerCfg := &container.Config{
		Image: cfg.Image,
		Env:   envList,
		ExposedPorts: portSet,
		Labels: map[string]string{
			"luxview.app":          app.Subdomain,
			"luxview.app.id":       app.ID.String(),
			"luxview.app.type":     "game",
			"luxview.game.template": cfg.TemplateID,
			"luxview.managed":      "true",
		},
	}

	hostCfg := &container.HostConfig{
		PortBindings:  portMap,
		RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
		Resources: container.Resources{
			NanoCPUs: nanoCPUs,
			Memory:   memory,
		},
		Mounts: mounts,
	}

	_ = s.docker.StopContainer(ctx, containerName, 30)
	_ = s.docker.RemoveContainer(ctx, containerName, true)

	containerID, err := s.docker.CreateContainer(ctx, containerCfg, hostCfg, &network.NetworkingConfig{}, containerName)
	if err != nil {
		return "", fmt.Errorf("create game container: %w", err)
	}

	if err := s.docker.StartContainer(ctx, containerID); err != nil {
		_ = s.docker.RemoveContainer(ctx, containerID, true)
		return "", fmt.Errorf("start game container: %w", err)
	}

	if s.gameNetwork != "" {
		if err := s.docker.ConnectNetwork(ctx, s.gameNetwork, containerID); err != nil {
			log.Warn().Err(err).Str("network", s.gameNetwork).Msg("failed to connect game container to network")
		}
	}

	log.Info().Str("container", containerID[:12]).Str("app", app.Subdomain).Msg("game server started")
	return containerID, nil
}

// QueryStatus queries live player count via the A2S Steam protocol.
func (s *GameServerService) QueryStatus(ctx context.Context, cfg *model.GameServerConfig, serverIP string) (*model.GameServerStatus, error) {
	if cfg.QueryPort == 0 {
		return &model.GameServerStatus{Running: true}, nil
	}
	addr := net.JoinHostPort(serverIP, strconv.Itoa(cfg.QueryPort))
	info, err := queryA2S(addr)
	if err != nil {
		return &model.GameServerStatus{Running: false}, nil
	}
	return &model.GameServerStatus{
		Running:    true,
		Players:    info.players,
		MaxPlayers: info.maxPlayers,
	}, nil
}

// QueryPlayers returns the list of connected players via the A2S_PLAYER protocol.
func (s *GameServerService) QueryPlayers(ctx context.Context, cfg *model.GameServerConfig, serverIP string) ([]model.PlayerInfo, error) {
	if cfg.QueryPort == 0 {
		return nil, fmt.Errorf("query port not configured")
	}
	addr := net.JoinHostPort(serverIP, strconv.Itoa(cfg.QueryPort))
	return queryA2SPlayers(addr)
}

// CountConnections counts established TCP connections inside the container whose
// local port is in the given set. Used to estimate online players for emulators
// without a query protocol (e.g. OpenMU), by counting connections on the game
// server ports. Reads /proc/net/tcp(6) via a container exec.
func (s *GameServerService) CountConnections(ctx context.Context, containerNameOrID string, ports map[int]bool) (int, error) {
	out, err := s.docker.ContainerExec(ctx, containerNameOrID, []string{"cat", "/proc/net/tcp", "/proc/net/tcp6"})
	if err != nil {
		return 0, err
	}

	const tcpStateEstablished = "01"
	count := 0
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		// Layout: "sl local_address rem_address st ...". Skip header/short lines.
		if len(fields) < 4 || fields[3] != tcpStateEstablished {
			continue
		}
		localAddr := fields[1]
		idx := strings.LastIndex(localAddr, ":")
		if idx < 0 {
			continue
		}
		port, err := strconv.ParseInt(localAddr[idx+1:], 16, 32)
		if err != nil {
			continue
		}
		if ports[int(port)] {
			count++
		}
	}
	return count, nil
}

// GetTemplates returns all available game server templates.
func GetGameTemplates() []model.GameTemplate {
	return []model.GameTemplate{vrisingTemplate(), openmuTemplate(), muemuTemplate(), rakionTemplate()}
}

func GetGameTemplate(id string) *model.GameTemplate {
	for _, t := range GetGameTemplates() {
		if t.ID == id {
			tCopy := t
			return &tCopy
		}
	}
	return nil
}

// --- A2S Steam server info protocol ---

type a2sInfo struct {
	players    int
	maxPlayers int
}

func queryA2S(addr string) (*a2sInfo, error) {
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

	// handle challenge response
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
		return nil, fmt.Errorf("unexpected A2S response type: %x", buf[4])
	}

	r := bytes.NewReader(buf[5:])
	r.ReadByte() // protocol version
	for i := 0; i < 4; i++ {
		for b, _ := r.ReadByte(); b != 0; b, _ = r.ReadByte() {
		} // skip null-terminated strings
	}
	r.ReadByte()               // appID low
	r.ReadByte()               // appID high
	players, _ := r.ReadByte()
	maxPlayers, _ := r.ReadByte()
	return &a2sInfo{players: int(players), maxPlayers: int(maxPlayers)}, nil
}

// --- A2S_PLAYER query ---

func queryA2SPlayers(addr string) ([]model.PlayerInfo, error) {
	conn, err := net.DialTimeout("udp", addr, 3*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second)) //nolint:errcheck

	// Step 1: Send A2S_PLAYER challenge request (0x55 + 0xFFFFFFFF)
	challengeReq := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x55, 0xFF, 0xFF, 0xFF, 0xFF}
	if _, err := conn.Write(challengeReq); err != nil {
		return nil, err
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	buf = buf[:n]

	// Expect challenge response: header(4) + 0x41 + challenge(4)
	if len(buf) < 9 || buf[4] != 0x41 {
		return nil, fmt.Errorf("unexpected A2S_PLAYER challenge response")
	}

	// Step 2: Send A2S_PLAYER with the challenge token
	playerReq := append([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x55}, buf[5:9]...)
	if _, err := conn.Write(playerReq); err != nil {
		return nil, err
	}

	buf = buf[:cap(buf)]
	n, err = conn.Read(buf)
	if err != nil {
		return nil, err
	}
	buf = buf[:n]

	// Response: header(4) + 0x44 + numPlayers(1) + player data
	if len(buf) < 6 || buf[4] != 0x44 {
		return nil, fmt.Errorf("unexpected A2S_PLAYER response type: %x", buf[4])
	}

	numPlayers := int(buf[5])
	r := bytes.NewReader(buf[6:])

	players := make([]model.PlayerInfo, 0, numPlayers)
	for i := 0; i < numPlayers; i++ {
		r.ReadByte() // index

		// Read null-terminated name
		var nameBytes []byte
		for {
			b, err := r.ReadByte()
			if err != nil || b == 0 {
				break
			}
			nameBytes = append(nameBytes, b)
		}

		var score int32
		binary.Read(r, binary.LittleEndian, &score)

		var duration float32
		binary.Read(r, binary.LittleEndian, &duration)

		name := string(nameBytes)
		if name == "" {
			name = fmt.Sprintf("Jogador %d", i+1)
		}

		players = append(players, model.PlayerInfo{
			Name:     name,
			Score:    int(score),
			Duration: math.Round(float64(duration)*100) / 100,
		})
	}

	return players, nil
}

// --- V Rising template definition ---

func sel(pairs ...string) []model.SelectOptionDef {
	opts := make([]model.SelectOptionDef, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		opts = append(opts, model.SelectOptionDef{Value: pairs[i], Label: pairs[i+1]})
	}
	return opts
}

func vrisingTemplate() model.GameTemplate {
	yesNo := sel("true", "Sim", "false", "Não")
	return model.GameTemplate{
		ID:               "vrising",
		DisplayName:      "V Rising Dedicated Server",
		Description:      "Servidor dedicado de V Rising com configuração completa.",
		Protocol:         "udp",
		DefaultGamePort:  27015,
		DefaultQueryPort: 27016,
		DefaultImage:     "luxview-cloud-vrising:latest",
		SupportsQuery:    true,
		DefaultVolumes: []model.GameVolume{
			{MountPath: "/vrising-server"}, // Steam-installed server binaries
			{MountPath: "/vrising-data"},   // World saves + Wine prefix
		},
		ConfigFields: []model.ConfigFieldDef{
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
			{Key: "VRGAME_GAME_MODE_TYPE", Label: "Modo de Jogo", Type: "select", Section: "Modo de Jogo", Options: sel("PvP", "PvP", "PvE", "PvE")},
			{Key: "VRGAME_GAME_DIFFICULTY", Label: "Dificuldade", Type: "select", Section: "Modo de Jogo", Options: sel("Normal", "Normal", "Brutal", "Brutal")},
			{Key: "VRISING_DIFFICULTY_PRESET", Label: "Preset de Dificuldade", Type: "text", Placeholder: "Opcional (sobrescreve preset)", Section: "Modo de Jogo"},
			{Key: "VRGAME_CASTLE_DAMAGE_MODE", Label: "Dano em Castelos", Type: "select", Section: "Modo de Jogo", Options: sel("Never", "Nunca", "Always", "Sempre", "TimeRestricted", "Horário restrito")},
			{Key: "VRGAME_PLAYER_DAMAGE_MODE", Label: "Dano entre Jogadores", Type: "select", Section: "Modo de Jogo", Options: sel("Always", "Sempre", "TimeRestricted", "Horário restrito")},
			{Key: "VRGAME_CASTLE_HEART_DAMAGE_MODE", Label: "Dano ao Coração do Castelo", Type: "select", Section: "Modo de Jogo", Options: sel("CanBeDestroyedByPlayers", "Pode ser destruído", "CanBeSeizedOrDestroyedByPlayers", "Pode ser tomado ou destruído", "CannotBeDestroyed", "Indestrutível")},
			{Key: "VRGAME_PVP_PROTECTION_MODE", Label: "Proteção PvP", Type: "select", Section: "Modo de Jogo", Options: sel("Disabled", "Desativado", "VeryShort", "Muito curto", "Short", "Curto", "Medium", "Médio")},
			{Key: "VRGAME_DEATH_CONTAINER_PERMISSION", Label: "Loot na morte", Type: "select", Section: "Modo de Jogo", Options: sel("Anyone", "Qualquer um", "KillerOnly", "Apenas quem matou", "OwnerOnly", "Apenas o dono")},
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
			{Key: "VRGAME_ANNOUNCE_VAMPIRE_KILLS", Label: "Anunciar mortes no chat", Type: "select", Section: "Modo de Jogo", Options: yesNo},
			{Key: "VRGAME_PVP_VAMPIRE_RESPAWN", Label: "Cooldown de respawn PvP", Type: "number", Placeholder: "1.0", Section: "Modo de Jogo"},
			{Key: "VRGAME_COOLDOWN_GLOBAL", Label: "Cooldown global de habilidades", Type: "number", Placeholder: "1.0", Section: "Modo de Jogo"},
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
			{Key: "VRGAME_DROP_RATE_MISSIONS", Label: "Drop em Missões", Type: "number", Placeholder: "1.0", Section: "Taxas"},
			{Key: "VRGAME_JOURNAL_QUEST_STACKS", Label: "Stacks de Quest do Journal", Type: "number", Placeholder: "1.0", Section: "Taxas"},
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
			{Key: "VRGAME_SPAWN_RATE", Label: "Taxa de Spawn de Inimigos", Type: "number", Placeholder: "1.0", Section: "Mundo"},
			// Jogador (Vampiro) — stats do personagem do jogador
			{Key: "VRGAME_VAMPIRE_HEALTH", Label: "Vida do Jogador", Type: "number", Placeholder: "1.0", Section: "Jogador"},
			{Key: "VRGAME_VAMPIRE_DAMAGE_RECEIVED", Label: "Dano Recebido pelo Jogador", Type: "number", Placeholder: "1.0", Section: "Jogador"},
			{Key: "VRGAME_VAMPIRE_PHYSICAL_POWER", Label: "Poder Físico do Jogador", Type: "number", Placeholder: "1.0", Section: "Jogador"},
			{Key: "VRGAME_VAMPIRE_SPELL_POWER", Label: "Poder Mágico do Jogador", Type: "number", Placeholder: "1.0", Section: "Jogador"},
			{Key: "VRGAME_VAMPIRE_RESOURCE_POWER", Label: "Poder de Coleta de Recurso", Type: "number", Placeholder: "1.0", Section: "Jogador"},
			{Key: "VRGAME_VAMPIRE_SIEGE_POWER", Label: "Poder de Cerco (dano em castelo)", Type: "number", Placeholder: "1.0", Section: "Jogador"},
			// Inimigos & Bosses — NPCs comuns e V-Blood (chefes)
			{Key: "VRGAME_UNIT_HEALTH", Label: "Vida dos NPCs/Inimigos", Type: "number", Placeholder: "1.0", Section: "Inimigos & Bosses"},
			{Key: "VRGAME_UNIT_POWER", Label: "Poder/Dano dos NPCs/Inimigos", Type: "number", Placeholder: "1.0", Section: "Inimigos & Bosses"},
			{Key: "VRGAME_UNIT_LEVEL_INCREASE", Label: "Bônus de Nível de NPCs/Inimigos", Type: "number", Placeholder: "0", Section: "Inimigos & Bosses"},
			{Key: "VRGAME_VBLOOD_HEALTH", Label: "Vida dos Bosses (V-Blood)", Type: "number", Placeholder: "1.0", Section: "Inimigos & Bosses"},
			{Key: "VRGAME_VBLOOD_POWER", Label: "Poder/Dano dos Bosses (V-Blood)", Type: "number", Placeholder: "1.0", Section: "Inimigos & Bosses"},
			{Key: "VRGAME_VBLOOD_LEVEL_INCREASE", Label: "Bônus de Nível dos Bosses (V-Blood)", Type: "number", Placeholder: "0", Section: "Inimigos & Bosses"},
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
	}
}
