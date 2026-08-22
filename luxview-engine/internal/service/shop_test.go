package service

import (
	"strings"
	"testing"

	"github.com/luxview/engine/internal/model"
)

func TestShopCatalogFiltersByTemplate(t *testing.T) {
	got := ShopCatalog("rakion")
	if len(got) == 0 {
		t.Fatal("expected rakion items")
	}
	for _, item := range got {
		if item.TemplateID != "rakion" {
			t.Fatalf("leaked %s", item.ID)
		}
	}
	if len(ShopCatalog("priston")) != 0 {
		t.Fatal("priston has no grant catalog yet")
	}
}

func TestRakionGrantSQL(t *testing.T) {
	item, ok := ShopItemByID("rakion-gold-10k")
	if !ok {
		t.Fatal("missing item")
	}
	sql, err := rakionGrantSQL(item, "testando")
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"rakion.usergameinfo", "gold = gold + 10000", "testando"} {
		if !strings.Contains(sql, part) {
			t.Fatalf("missing %q in %s", part, sql)
		}
	}
	cash, _ := ShopItemByID("rakion-cash-200")
	sql, err = rakionGrantSQL(cash, "testando")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "rakion.cash") || !strings.Contains(sql, "cash + 200") {
		t.Fatalf("cash sql: %s", sql)
	}
}

func TestTibiaGrantSQL(t *testing.T) {
	coins, _ := ShopItemByID("tibia-coins-25")
	sql, err := tibiaGrantSQL(coins, "Joao", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "canary.accounts") || !strings.Contains(sql, "coins = coins + 25") {
		t.Fatalf("coins sql: %s", sql)
	}
	bank, _ := ShopItemByID("tibia-gold-banco")
	if _, err := tibiaGrantSQL(bank, "Joao", &model.PlayerGameLink{InGameNick: "joao@luxviewot.com"}); err == nil {
		t.Fatal("bank grant must require a character nick")
	}
	sql, err = tibiaGrantSQL(bank, "Joao", &model.PlayerGameLink{InGameNick: "Joao Mage"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "canary.players") || !strings.Contains(sql, "Joao Mage") {
		t.Fatalf("bank sql: %s", sql)
	}
}

func TestMetinGrantSQL(t *testing.T) {
	item, _ := ShopItemByID("metin2-yang-1m")
	sql, err := metinGrantSQL(item, "testando")
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"player.player", "p.gold = p.gold + 1000000", "account.account", "testando"} {
		if !strings.Contains(sql, part) {
			t.Fatalf("missing %q in %s", part, sql)
		}
	}
}
