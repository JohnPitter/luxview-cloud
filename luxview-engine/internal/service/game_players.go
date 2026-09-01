package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/luxview/engine/internal/model"
)

func (s *GameServerService) queryGamePlayers(ctx context.Context, cfg *model.GameServerConfig, serverIP string) ([]model.PlayerInfo, error) {
	if s == nil || s.docker == nil || cfg == nil {
		return nil, fmt.Errorf("docker indisponível")
	}
	container := serverIP
	switch cfg.TemplateID {
	case "tibia":
		return s.queryTibiaPlayers(ctx, container, cfg)
	case "metin2":
		return s.queryMetinPlayers(ctx, container, cfg)
	case "muemu":
		return s.queryMuEmuPlayers(ctx, container, cfg)
	case "openmu":
		return s.queryOpenMUPlayers(ctx, container, cfg)
	case "rakion":
		return s.queryRakionPlayers(ctx, container, cfg)
	case "priston":
		return s.queryPristonPlayers(ctx, container)
	default:
		return nil, fmt.Errorf("sem roster SQL para %s", cfg.TemplateID)
	}
}

func (s *GameServerService) queryTibiaPlayers(ctx context.Context, container string, cfg *model.GameServerConfig) ([]model.PlayerInfo, error) {
	out, err := s.execMySQL(ctx, container, cfg, "tibia",
		`SELECT p.name, p.level, p.vocation, CONCAT(p.town_id, ':', p.posx, ',', p.posy, ',', p.posz)
FROM canary.players_online o
INNER JOIN canary.players p ON p.id = o.player_id`)
	if err != nil {
		return nil, err
	}
	return parseRoster(out, func(cols []string) model.PlayerInfo {
		level, _ := strconv.Atoi(cols[1])
		voc, _ := strconv.Atoi(cols[2])
		townID, coords, _ := strings.Cut(cols[3], ":")
		tid, _ := strconv.Atoi(townID)
		loc := tibiaTownName(tid)
		if coords != "" {
			loc = loc + " (" + coords + ")"
		}
		return model.PlayerInfo{
			Name: cols[0], Character: cols[0], Class: tibiaVocationName(voc),
			Location: loc, Level: level,
		}
	}), nil
}

func (s *GameServerService) queryMetinPlayers(ctx context.Context, container string, cfg *model.GameServerConfig) ([]model.PlayerInfo, error) {
	out, err := s.execMySQL(ctx, container, cfg, "metin2",
		`SELECT name, level, job, map_index FROM player.player
WHERE last_play >= DATE_SUB(NOW(), INTERVAL 10 MINUTE)`)
	if err != nil {
		return nil, err
	}
	return parseRoster(out, func(cols []string) model.PlayerInfo {
		level, _ := strconv.Atoi(cols[1])
		job, _ := strconv.Atoi(cols[2])
		mapID, _ := strconv.Atoi(cols[3])
		return model.PlayerInfo{
			Name: cols[0], Character: cols[0], Class: metinJobName(job),
			Location: metinMapName(mapID), Level: level,
		}
	}), nil
}

func (s *GameServerService) queryMuEmuPlayers(ctx context.Context, container string, cfg *model.GameServerConfig) ([]model.PlayerInfo, error) {
	queries := []string{
		"SELECT c.Name, c.Level, c.Class, c.Map FROM `Character` c INNER JOIN Account a ON a.Id = c.AccountId WHERE a.IsConnected = 1",
		"SELECT Name, Level, Class, Map FROM `Character` WHERE IsConnected = 1",
		"SELECT Name, Level, Class, Map FROM `Character` WHERE UNIX_TIMESTAMP(NOW()) - UNIX_TIMESTAMP(Date) < 600",
	}
	var last error
	for _, q := range queries {
		out, err := s.execMySQL(ctx, container, cfg, "muemu", q)
		if err != nil {
			last = err
			continue
		}
		return parseRoster(out, mapMuPlayer), nil
	}
	return nil, last
}

func mapMuPlayer(cols []string) model.PlayerInfo {
	level, _ := strconv.Atoi(cols[1])
	classID, _ := strconv.Atoi(cols[2])
	mapID, _ := strconv.Atoi(cols[3])
	return model.PlayerInfo{
		Name: cols[0], Character: cols[0], Class: muClassName(classID),
		Location: muMapName(mapID), Level: level,
	}
}

type openMUPlayersResponse struct {
	Players []struct {
		Character         string `json:"character"`
		Account           string `json:"account"`
		Class             string `json:"class"`
		Level             int    `json:"level"`
		Location          string `json:"location"`
		ServerID          int    `json:"server_id"`
		ServerDescription string `json:"server_description"`
	} `json:"players"`
}

func mapOpenMUHTTPPlayer(p struct {
	Character         string `json:"character"`
	Account           string `json:"account"`
	Class             string `json:"class"`
	Level             int    `json:"level"`
	Location          string `json:"location"`
	ServerID          int    `json:"server_id"`
	ServerDescription string `json:"server_description"`
}) model.PlayerInfo {
	name := strings.TrimSpace(p.Character)
	if name == "" {
		name = strings.TrimSpace(p.Account)
	}
	return model.PlayerInfo{
		Name:      name,
		Character: strings.TrimSpace(p.Character),
		Account:   strings.TrimSpace(p.Account),
		Class:     strings.TrimSpace(p.Class),
		Location:  strings.TrimSpace(p.Location),
		Level:     p.Level,
		Server:    formatOpenMUServer(p.ServerID, p.ServerDescription),
	}
}

func parseOpenMUPlayersJSON(body []byte) ([]model.PlayerInfo, error) {
	var payload openMUPlayersResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	players := make([]model.PlayerInfo, 0, len(payload.Players))
	for _, p := range payload.Players {
		if strings.TrimSpace(p.Character) == "" && strings.TrimSpace(p.Account) == "" {
			continue
		}
		players = append(players, mapOpenMUHTTPPlayer(p))
	}
	return players, nil
}

func (s *GameServerService) queryOpenMUPlayers(ctx context.Context, container string, cfg *model.GameServerConfig) ([]model.PlayerInfo, error) {
	body, err := s.fetchOpenMUPlayersJSON(ctx, container)
	if err != nil {
		return nil, err
	}
	return parseOpenMUPlayersJSON(body)
}

func (s *GameServerService) fetchOpenMUPlayersJSON(ctx context.Context, container string) ([]byte, error) {
	url := fmt.Sprintf("http://%s:%d/api/players", container, OpenMUAdminPanelPort)
	if s.docker != nil {
		out, execErr := s.docker.ContainerExec(ctx, container, []string{
			"curl", "-fsS", url,
		})
		if execErr == nil {
			return []byte(strings.TrimSpace(out)), nil
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openmu /api/players: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (s *GameServerService) queryRakionPlayers(ctx context.Context, container string, cfg *model.GameServerConfig) ([]model.PlayerInfo, error) {
	queries := []string{
		`SELECT name, IFNULL(level,0), IFNULL(class,''), IFNULL(channel,'partida') FROM rakion.usergameinfo WHERE online = 1`,
		`SELECT name, IFNULL(level,0), '', 'online' FROM rakion.usergameinfo WHERE lastlogin >= DATE_SUB(NOW(), INTERVAL 15 MINUTE)`,
	}
	var last error
	for _, q := range queries {
		out, err := s.execMySQL(ctx, container, cfg, "rakion", q)
		if err != nil {
			last = err
			continue
		}
		return parseRoster(out, func(cols []string) model.PlayerInfo {
			level, _ := strconv.Atoi(cols[1])
			className := cols[2]
			if className == "" {
				className = "Combatente"
			}
			return model.PlayerInfo{
				Name: cols[0], Character: cols[0], Class: className,
				Location: cols[3], Level: level,
			}
		}), nil
	}
	return nil, last
}

func (s *GameServerService) queryPristonPlayers(ctx context.Context, container string) ([]model.PlayerInfo, error) {
	out, err := s.docker.ContainerExec(ctx, container, []string{"wget", "-qO-", "http://127.0.0.1:5080/players"})
	if err != nil {
		out, err = s.docker.ContainerExec(ctx, container, []string{"curl", "-fsS", "http://127.0.0.1:5080/players"})
	}
	if err != nil {
		return nil, err
	}
	var payload struct {
		Players []struct {
			Account   string `json:"account"`
			Character string `json:"character"`
			Level     int    `json:"level"`
			Location  string `json:"location"`
		} `json:"players"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		return nil, err
	}
	players := make([]model.PlayerInfo, 0, len(payload.Players))
	for _, p := range payload.Players {
		name := p.Character
		if name == "" {
			name = p.Account
		}
		loc := p.Location
		if loc == "" {
			loc = "Lobby / seleção"
		}
		players = append(players, model.PlayerInfo{
			Name: name, Character: p.Character, Account: p.Account,
			Location: loc, Level: p.Level,
		})
	}
	return players, nil
}

func (s *GameServerService) execMySQL(ctx context.Context, container string, cfg *model.GameServerConfig, templateID, sql string) (string, error) {
	pass := mysqlRootPassword(templateID, cfg.ConfigFields)
	env := []string{"MYSQL_PWD=" + pass}
	out, err := s.docker.ContainerExec(ctx, container, []string{"mysql", "-uroot", "-N", "-B", "-e", sql}, env...)
	if err == nil {
		return out, nil
	}
	return s.docker.ContainerExec(ctx, container, []string{"mariadb", "-uroot", "-N", "-B", "-e", sql}, env...)
}

func (s *GameServerService) execPostgres(ctx context.Context, container string, cfg *model.GameServerConfig, sql string) (string, error) {
	pass := "openmu"
	host := "127.0.0.1"
	if cfg != nil && cfg.ConfigFields != nil {
		if v := strings.TrimSpace(cfg.ConfigFields["POSTGRES_PASSWORD"]); v != "" {
			pass = v
		}
		if v := strings.TrimSpace(cfg.ConfigFields["PGHOST"]); v != "" {
			host = v
		}
	}
	env := []string{"PGPASSWORD=" + pass}
	return s.docker.ContainerExec(ctx, container, []string{
		"psql", "-h", host, "-U", "postgres", "-d", "openmu", "-A", "-t", "-F", "\t", "-c", sql,
	}, env...)
}

func parseRoster(out string, mapRow func([]string) model.PlayerInfo) []model.PlayerInfo {
	players := make([]model.PlayerInfo, 0)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 4 {
			continue
		}
		players = append(players, mapRow(cols))
	}
	return players
}
