package service

import (
	"bytes"
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
	if players, err := s.queryOpenMUPlayersHTTP(ctx, container); err == nil {
		return players, nil
	}
	return s.queryOpenMUPlayersFromStatus(ctx, container, cfg)
}

func (s *GameServerService) queryOpenMUPlayersHTTP(ctx context.Context, container string) ([]model.PlayerInfo, error) {
	body, err := s.fetchOpenMUAdminPath(ctx, container, "/api/players")
	if err != nil {
		return nil, err
	}
	return parseOpenMUPlayersJSON(body)
}

// openMUPlayersByNamesSQL enriches live names from /api/status with persisted metadata.
const openMUPlayersByNamesSQL = `SELECT c."Name",
       COALESCE((
         SELECT ROUND(sa."Value")::integer
         FROM data."StatAttribute" sa
         JOIN config."AttributeDefinition" ad ON ad."Id" = sa."DefinitionId"
         WHERE sa."CharacterId" = c."Id" AND ad."Designation" = 'Level'
         LIMIT 1
       ), 1),
       COALESCE(cc."Name", ''),
       COALESCE(m."Name", ''),
       COALESCE(gs."ServerID", 0),
       COALESCE(gs."Description", '')
FROM data."Character" c
LEFT JOIN config."CharacterClass" cc ON cc."Id" = c."CharacterClassId"
LEFT JOIN config."GameMapDefinition" m ON m."Id" = c."CurrentMapId"
LEFT JOIN LATERAL (
  SELECT g."ServerID", g."Description"
  FROM config."GameServerDefinition" g
  WHERE g."GameConfigurationId" = COALESCE(m."GameConfigurationId", cc."GameConfigurationId")
  ORDER BY g."ServerID"
  LIMIT 1
) gs ON true
JOIN data."Account" a ON a."Id" = c."AccountId"
WHERE c."Name" IN (%s)
  AND COALESCE(a."IsBot", false) = false
  AND COALESCE(a."IsTemplate", false) = false`

func mapOpenMUPlayer(cols []string) model.PlayerInfo {
	level, _ := strconv.Atoi(cols[1])
	info := model.PlayerInfo{
		Name: cols[0], Character: cols[0], Class: cols[2],
		Location: cols[3], Level: level,
	}
	if len(cols) >= 6 {
		sid, _ := strconv.Atoi(cols[4])
		info.Server = formatOpenMUServer(sid, cols[5])
	}
	return info
}

func (s *GameServerService) queryOpenMUPlayersFromStatus(ctx context.Context, container string, cfg *model.GameServerConfig) ([]model.PlayerInfo, error) {
	body, err := s.fetchOpenMUAdminPath(ctx, container, "/api/status")
	if err != nil {
		return nil, err
	}
	names := parseOpenMUStatusNames(body)
	if len(names) == 0 {
		return []model.PlayerInfo{}, nil
	}
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		quoted = append(quoted, "'"+strings.ReplaceAll(name, "'", "''")+"'")
	}
	if len(quoted) == 0 {
		return []model.PlayerInfo{}, nil
	}
	out, err := s.execPostgres(ctx, container, cfg, fmt.Sprintf(openMUPlayersByNamesSQL, strings.Join(quoted, ",")))
	if err == nil {
		if players := parseRoster(out, mapOpenMUPlayer); len(players) > 0 {
			return players, nil
		}
	}
	players := make([]model.PlayerInfo, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		players = append(players, model.PlayerInfo{Name: name, Character: name})
	}
	return players, nil
}

type openMUStatusResponse struct {
	PlayersList []string `json:"playersList"`
}

func parseOpenMUStatusNames(body []byte) []string {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil
	}
	if body[0] == '"' {
		var encoded string
		if err := json.Unmarshal(body, &encoded); err == nil {
			body = []byte(encoded)
		}
	}
	var payload openMUStatusResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	return payload.PlayersList
}

func (s *GameServerService) fetchOpenMUAdminPath(ctx context.Context, container, path string) ([]byte, error) {
	if s.docker != nil {
		for _, port := range []int{5000, OpenMUAdminPanelPort} {
			localURL := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
			out, execErr := s.docker.ContainerExec(ctx, container, []string{"curl", "-fsS", localURL})
			if execErr == nil {
				trimmed := strings.TrimSpace(out)
				if trimmed != "" {
					return []byte(trimmed), nil
				}
			}
		}
	}
	url := fmt.Sprintf("http://%s:%d%s", container, OpenMUAdminPanelPort, path)
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
		return nil, fmt.Errorf("openmu %s: HTTP %d", path, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (s *GameServerService) fetchOpenMUPlayersJSON(ctx context.Context, container string) ([]byte, error) {
	return s.fetchOpenMUAdminPath(ctx, container, "/api/players")
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
