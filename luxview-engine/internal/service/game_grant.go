package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/pkg/logger"
)

func (g *GameAccount) Grant(ctx context.Context, player *model.PlayerAccount, appID uuid.UUID, item model.ShopItem) error {
	app, err := g.appRepo.FindByID(ctx, appID)
	if err != nil || app == nil {
		return fmt.Errorf("jogo não encontrado")
	}
	cfg := app.GameConfig
	if cfg == nil && g.gameConfigRepo != nil {
		cfg, err = g.gameConfigRepo.GetByAppID(ctx, app.ID)
		if err != nil {
			return err
		}
	}
	if cfg == nil || cfg.TemplateID != item.TemplateID {
		return fmt.Errorf("template do jogo não bate com o item")
	}
	link, err := g.playerRepo.FindLink(ctx, player.ID, appID)
	if err != nil {
		return err
	}
	sql, err := gameGrantSQL(item, player.Username, link)
	if err != nil {
		return err
	}
	if err := g.execSQL(ctx, ContainerName(app.Subdomain), cfg.TemplateID, cfg.ConfigFields, sql); err != nil {
		log := logger.With("game-grant")
		log.Warn().Err(err).Str("item", item.ID).Str("template", cfg.TemplateID).Msg("grant item")
		return err
	}
	return nil
}

func gameGrantSQL(item model.ShopItem, username string, link *model.PlayerGameLink) (string, error) {
	if item.Amount <= 0 {
		return "", fmt.Errorf("quantidade inválida")
	}
	switch item.TemplateID {
	case "rakion":
		return rakionGrantSQL(item, username)
	case "tibia":
		return tibiaGrantSQL(item, username, link)
	case "metin2":
		return metinGrantSQL(item, username)
	default:
		return "", fmt.Errorf("este jogo ainda não recebe itens da loja")
	}
}

func rakionGrantSQL(item model.ShopItem, username string) (string, error) {
	login := RakionLogin(username)
	if login == "" {
		return "", fmt.Errorf("usuário inválido para o Rakion")
	}
	id := mysqlQuote(login)
	switch item.Grant {
	case model.ShopGrantGold:
		return fmt.Sprintf(`UPDATE rakion.usergameinfo SET gold = gold + %d WHERE name = %s;`, item.Amount, id), nil
	case model.ShopGrantCash:
		return fmt.Sprintf(`INSERT INTO rakion.cash (id, cash) VALUES (%s, %d) ON DUPLICATE KEY UPDATE cash = cash + %d;`, id, item.Amount, item.Amount), nil
	default:
		return "", fmt.Errorf("grant Rakion desconhecido")
	}
}

func tibiaGrantSQL(item model.ShopItem, username string, link *model.PlayerGameLink) (string, error) {
	email := mysqlQuote(TibiaEmail(username))
	switch item.Grant {
	case model.ShopGrantCoins:
		return fmt.Sprintf(`UPDATE canary.accounts SET coins = coins + %d WHERE name = %s;`, item.Amount, email), nil
	case model.ShopGrantPremDays:
		return fmt.Sprintf(`UPDATE canary.accounts SET premdays = premdays + %d WHERE name = %s;`, item.Amount, email), nil
	case model.ShopGrantBank:
		charName := tibiaCharacterName(link)
		if charName == "" {
			return "", fmt.Errorf("crie um personagem Tibia antes de comprar gold")
		}
		return fmt.Sprintf(`UPDATE canary.players SET balance = balance + %d WHERE name = %s;`, item.Amount, mysqlQuote(charName)), nil
	default:
		return "", fmt.Errorf("grant Tibia desconhecido")
	}
}

func metinGrantSQL(item model.ShopItem, username string) (string, error) {
	login := MetinLogin(username)
	if login == "" {
		return "", fmt.Errorf("usuário inválido para o Metin2")
	}
	if item.Grant != model.ShopGrantYang {
		return "", fmt.Errorf("grant Metin2 desconhecido")
	}
	return fmt.Sprintf(
		`UPDATE player.player p INNER JOIN account.account a ON a.id = p.account_id SET p.gold = p.gold + %d WHERE a.login = %s LIMIT 1;`,
		item.Amount, mysqlQuote(login),
	), nil
}

func tibiaCharacterName(link *model.PlayerGameLink) string {
	if link == nil {
		return ""
	}
	nick := strings.TrimSpace(link.InGameNick)
	if nick == "" || strings.Contains(nick, "@") {
		return ""
	}
	return nick
}
