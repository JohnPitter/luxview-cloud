package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
)

const (
	welcomeCash   int64 = 100
	welcomeReward int64 = 25
)

var shopCatalog = []model.ShopItem{
	{ID: "rakion-gold-10k", Name: "10.000 Gold", Description: "Gold creditado na conta Rakion.", TemplateID: "rakion", Currency: model.LedgerCash, Price: 40, Grant: model.ShopGrantGold, Amount: 10_000},
	{ID: "rakion-cash-200", Name: "200 Cash", Description: "Cash shop do Rakion.", TemplateID: "rakion", Currency: model.LedgerCash, Price: 80, Grant: model.ShopGrantCash, Amount: 200},
	{ID: "rakion-gold-premio", Name: "5.000 Gold", Description: "Pacote de prêmio Rakion.", TemplateID: "rakion", Currency: model.LedgerReward, Price: 20, Grant: model.ShopGrantGold, Amount: 5_000},
	{ID: "tibia-coins-25", Name: "25 Tibia Coins", Description: "Coins na conta Canary.", TemplateID: "tibia", Currency: model.LedgerCash, Price: 100, Grant: model.ShopGrantCoins, Amount: 25},
	{ID: "tibia-premdays-7", Name: "7 dias VIP", Description: "Premium days na conta Tibia.", TemplateID: "tibia", Currency: model.LedgerCash, Price: 150, Grant: model.ShopGrantPremDays, Amount: 7},
	{ID: "tibia-gold-banco", Name: "100.000 Gold", Description: "Gold no banco do personagem.", TemplateID: "tibia", Currency: model.LedgerReward, Price: 30, Grant: model.ShopGrantBank, Amount: 100_000},
	{ID: "metin2-yang-1m", Name: "1.000.000 Yang", Description: "Yang no primeiro personagem da conta.", TemplateID: "metin2", Currency: model.LedgerCash, Price: 50, Grant: model.ShopGrantYang, Amount: 1_000_000},
}

type itemGranter interface {
	Grant(ctx context.Context, player *model.PlayerAccount, appID uuid.UUID, item model.ShopItem) error
}

type Shop struct {
	players *Player
	grants  itemGranter
	repo    *repository.PlayerRepo
}

func NewShop(players *Player, grants itemGranter, repo *repository.PlayerRepo) *Shop {
	return &Shop{players: players, grants: grants, repo: repo}
}

func ShopCatalog(templateID string) []model.ShopItem {
	want := strings.TrimSpace(templateID)
	out := make([]model.ShopItem, 0, len(shopCatalog))
	for _, item := range shopCatalog {
		if want != "" && item.TemplateID != want {
			continue
		}
		out = append(out, item)
	}
	return out
}

func ShopItemByID(id string) (model.ShopItem, bool) {
	for _, item := range shopCatalog {
		if item.ID == id {
			return item, true
		}
	}
	return model.ShopItem{}, false
}

func (s *Shop) Buy(ctx context.Context, player *model.PlayerAccount, appID uuid.UUID, itemID string) (*model.ShopBuyResult, error) {
	item, ok := ShopItemByID(itemID)
	if !ok {
		return nil, fmt.Errorf("item não encontrado")
	}
	link, err := s.repo.FindLink(ctx, player.ID, appID)
	if err != nil {
		return nil, err
	}
	if link == nil {
		return nil, fmt.Errorf("entre no jogo uma vez para criar a conta in-game")
	}
	if link.TemplateID != item.TemplateID {
		return nil, fmt.Errorf("este item é do %s", item.TemplateID)
	}

	acct, err := s.players.Debit(ctx, player.ID, item.Currency, item.Price, "shop:"+item.ID)
	if err != nil {
		return nil, err
	}
	if err := s.grants.Grant(ctx, acct, appID, item); err != nil {
		_, _ = s.players.Credit(ctx, player.ID, item.Currency, item.Price, "shop-refund:"+item.ID)
		return nil, fmt.Errorf("não consegui entregar o item no servidor")
	}
	_ = s.repo.CreateOrder(ctx, &model.ShopOrder{
		PlayerID: player.ID, AppID: appID, ItemID: item.ID,
		Currency: item.Currency, Price: item.Price, Status: "granted",
	})
	return &model.ShopBuyResult{Item: item, CashPoints: acct.CashPoints, RewardPoints: acct.RewardPoints}, nil
}
