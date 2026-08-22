package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	dockerclient "github.com/luxview/engine/pkg/docker"
	"github.com/luxview/engine/pkg/logger"
)

type GameAccount struct {
	docker         *dockerclient.Client
	appRepo        *repository.AppRepo
	gameConfigRepo *repository.GameServerConfigRepo
	playerRepo     *repository.PlayerRepo
}

func NewGameAccount(
	docker *dockerclient.Client,
	appRepo *repository.AppRepo,
	gameConfigRepo *repository.GameServerConfigRepo,
	playerRepo *repository.PlayerRepo,
) *GameAccount {
	return &GameAccount{
		docker: docker, appRepo: appRepo,
		gameConfigRepo: gameConfigRepo, playerRepo: playerRepo,
	}
}

type GameAccountInfo struct {
	TemplateID string `json:"template_id"`
	Login      string `json:"login"`
	Email      string `json:"email,omitempty"`
	Character  string `json:"character,omitempty"`
}

func (g *GameAccount) Provision(ctx context.Context, player *model.PlayerAccount, appID uuid.UUID, password, characterName, vocation string) (*GameAccountInfo, error) {
	if player == nil {
		return nil, fmt.Errorf("entre na conta LuxView")
	}
	if strings.TrimSpace(password) == "" {
		return nil, fmt.Errorf("senha ausente")
	}
	app, err := g.appRepo.FindByID(ctx, appID)
	if err != nil || app == nil {
		return nil, fmt.Errorf("jogo não encontrado")
	}
	if app.AppType != model.AppTypeGame {
		return nil, fmt.Errorf("app não é um jogo")
	}
	cfg := app.GameConfig
	if cfg == nil && g.gameConfigRepo != nil {
		cfg, err = g.gameConfigRepo.GetByAppID(ctx, app.ID)
		if err != nil {
			return nil, err
		}
	}
	if cfg == nil {
		return nil, fmt.Errorf("jogo sem template")
	}
	info, sql, err := gameAccountSQL(cfg.TemplateID, player.Username, password, characterName, vocation)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(sql) != "" {
		container := ContainerName(app.Subdomain)
		if err := g.execSQL(ctx, container, cfg.TemplateID, cfg.ConfigFields, sql); err != nil {
			log := logger.With("game-account")
			log.Warn().Err(err).Str("container", container).Str("template", cfg.TemplateID).Msg("provision game account")
			return nil, fmt.Errorf("não consegui criar a conta no servidor")
		}
	}
	nick := info.Login
	if info.Character != "" {
		nick = info.Character
	}
	_ = g.playerRepo.EnsureLink(ctx, &model.PlayerGameLink{
		PlayerID: player.ID, AppID: app.ID, TemplateID: cfg.TemplateID, InGameNick: nick,
	})
	return info, nil
}

func gameAccountSQL(templateID, username, password, characterName, vocation string) (*GameAccountInfo, string, error) {
	switch templateID {
	case "tibia":
		email := TibiaEmail(username)
		sql := fmt.Sprintf(
			`INSERT INTO canary.accounts (name, password, email, type, creation) VALUES (%s, %s, %s, 1, UNIX_TIMESTAMP()) ON DUPLICATE KEY UPDATE password = VALUES(password), email = VALUES(email);`,
			mysqlQuote(email), mysqlQuote(SHA1Hex(password)), mysqlQuote(email),
		)
		info := &GameAccountInfo{TemplateID: templateID, Login: email, Email: email}
		charName := strings.TrimSpace(characterName)
		voc := strings.TrimSpace(vocation)
		if charName == "" && voc == "" {
			return info, sql, nil
		}
		if charName == "" || voc == "" {
			return nil, "", fmt.Errorf("informe o nome e a vocação do personagem")
		}
		name, err := ParseTibiaCharacterName(charName)
		if err != nil {
			return nil, "", err
		}
		sample, err := TibiaVocationSample(voc)
		if err != nil {
			return nil, "", err
		}
		sql += fmt.Sprintf(
			`INSERT INTO canary.players (name, account_id, group_id, level, vocation, health, healthmax, experience, lookbody, lookfeet, lookhead, looklegs, looktype, lookaddons, maglevel, mana, manamax, manaspent, soul, town_id, posx, posy, posz, conditions, cap, sex, stamina, skill_fist, skill_club, skill_sword, skill_axe, skill_dist, skill_shielding, skill_fishing) SELECT %s, a.id, 1, s.level, s.vocation, s.health, s.healthmax, s.experience, s.lookbody, s.lookfeet, s.lookhead, s.looklegs, s.looktype, s.lookaddons, s.maglevel, s.mana, s.manamax, s.manaspent, s.soul, s.town_id, s.posx, s.posy, s.posz, s.conditions, s.cap, s.sex, s.stamina, s.skill_fist, s.skill_club, s.skill_sword, s.skill_axe, s.skill_dist, s.skill_shielding, s.skill_fishing FROM canary.accounts a INNER JOIN canary.players s ON s.name = %s WHERE a.name = %s AND NOT EXISTS (SELECT 1 FROM canary.players p WHERE p.account_id = a.id);`+
				`INSERT INTO canary.player_items (player_id, pid, sid, itemtype, count, attributes) SELECT np.id, i.pid, i.sid, i.itemtype, i.count, i.attributes FROM canary.players np INNER JOIN canary.accounts a ON a.id = np.account_id INNER JOIN canary.players sp ON sp.name = %s INNER JOIN canary.player_items i ON i.player_id = sp.id WHERE a.name = %s AND np.name = %s AND NOT EXISTS (SELECT 1 FROM canary.player_items x WHERE x.player_id = np.id);`,
			mysqlQuote(name), mysqlQuote(sample), mysqlQuote(email),
			mysqlQuote(sample), mysqlQuote(email), mysqlQuote(name),
		)
		info.Character = name
		return info, sql, nil
	case "metin2":
		login := MetinLogin(username)
		if login == "" {
			return nil, "", fmt.Errorf("usuário inválido para o Metin2")
		}
		email := TibiaEmail(username)
		sql := fmt.Sprintf(
			`INSERT INTO account.account (login, password, real_name, social_id, email, phone1, zipcode, status, securitycode, channel_company, create_time, language) VALUES (%s, %s, '', '1234567', %s, '', '', 'OK', '', '', NOW(), 2) ON DUPLICATE KEY UPDATE password = VALUES(password), email = VALUES(email);`,
			mysqlQuote(login), mysqlQuote(MySQLPassword(password)), mysqlQuote(email),
		)
		return &GameAccountInfo{TemplateID: templateID, Login: login, Email: email}, sql, nil
	case "rakion":
		id := RakionLogin(username)
		pass := RakionPassword(password)
		if id == "" {
			return nil, "", fmt.Errorf("usuário LuxView precisa de letras ou números para o Rakion")
		}
		if len([]rune(pass)) < 3 {
			return nil, "", fmt.Errorf("senha curta demais para o Rakion")
		}
		email := TibiaEmail(username)
		sql := fmt.Sprintf(
			`INSERT INTO rakion.user (id, password, e_mail) VALUES (%s, %s, %s) ON DUPLICATE KEY UPDATE password = VALUES(password), e_mail = VALUES(e_mail); INSERT INTO rakion.usergameinfo (name, gold, slot) SELECT %s, 10000, 4 WHERE NOT EXISTS (SELECT 1 FROM rakion.usergameinfo WHERE name = %s); INSERT IGNORE INTO rakion.cash (id, cash) VALUES (%s, 0);`,
			mysqlQuote(id), mysqlQuote(pass), mysqlQuote(email), mysqlQuote(id), mysqlQuote(id), mysqlQuote(id),
		)
		return &GameAccountInfo{TemplateID: templateID, Login: id, Email: email}, sql, nil
		case "priston":
			login := PristonLogin(username)
			if login == "" {
				return nil, "", fmt.Errorf("usuário LuxView precisa de letras ou números para o Priston Tale")
			}
			return &GameAccountInfo{TemplateID: templateID, Login: login, Email: TibiaEmail(username)}, "", nil
		case "muemu", "openmu":
			login := MuLogin(username)
			if login == "" {
				return nil, "", fmt.Errorf("usuário LuxView precisa de letras ou números para o MU")
			}
			return &GameAccountInfo{TemplateID: templateID, Login: login, Email: TibiaEmail(username)}, "", nil
		default:
		return nil, "", fmt.Errorf("este jogo ainda não cria conta pelo launcher")
	}
}

func mysqlRootPassword(templateID string, fields map[string]string) string {
	if fields != nil {
		if v := strings.TrimSpace(fields["MYSQL_ROOT_PASSWORD"]); v != "" {
			return v
		}
	}
	if templateID == "rakion" {
		return "123456"
	}
	if templateID == "muemu" {
		return "muemu"
	}
	return "root"
}

func (g *GameAccount) execSQL(ctx context.Context, container, templateID string, fields map[string]string, sql string) error {
	if g.docker == nil {
		return fmt.Errorf("docker indisponível")
	}
	env := []string{"MYSQL_PWD=" + mysqlRootPassword(templateID, fields)}
	if _, err := g.docker.ContainerExec(ctx, container, []string{"mysql", "-uroot", "-e", sql}, env...); err == nil {
		return nil
	}
	_, err := g.docker.ContainerExec(ctx, container, []string{"mariadb", "-uroot", "-e", sql}, env...)
	return err
}
