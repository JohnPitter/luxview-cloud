package service

import (
	"strings"
	"testing"
)

func TestGameAccountSQLTibiaAccountOnly(t *testing.T) {
	info, sql, err := gameAccountSQL("tibia", "Joao", "123456", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if info.Login != "joao@luxviewot.com" {
		t.Fatalf("login %s", info.Login)
	}
	if strings.Contains(sql, "canary.players") {
		t.Fatalf("account-only SQL created a character: %s", sql)
	}
}

func TestGameAccountSQLTibiaWithVocation(t *testing.T) {
	info, sql, err := gameAccountSQL("tibia", "Joao", "123456", "Joao Mage", "sorcerer")
	if err != nil {
		t.Fatal(err)
	}
	if info.Character != "Joao Mage" {
		t.Fatalf("character %s", info.Character)
	}
	for _, part := range []string{"canary.accounts", "canary.players", "Sorcerer Sample", "Joao Mage", "7c4a8d09ca3762af61e59520943dc26494f8941b"} {
		if !strings.Contains(sql, part) {
			t.Fatalf("missing %q in %s", part, sql)
		}
	}
}

func TestGameAccountSQLTibiaNeedsBothFields(t *testing.T) {
	if _, _, err := gameAccountSQL("tibia", "Joao", "123456", "Joao", ""); err == nil {
		t.Fatal("expected error when vocation is missing")
	}
}

func TestGameAccountSQLMetin(t *testing.T) {
	_, sql, err := gameAccountSQL("metin2", "testando", "123456", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"account.account", "*6BB4837EB74329105EE4568DDA7DC67ED2CA2AD9", "testando"} {
		if !strings.Contains(sql, part) {
			t.Fatalf("missing %q in %s", part, sql)
		}
	}
}

func TestGameAccountSQLPriston(t *testing.T) {
	info, sql, err := gameAccountSQL("priston", "testando", "Secret99", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if info.Login != "testando" {
		t.Fatalf("login %s", info.Login)
	}
	for _, part := range []string{"TGameUser", "testando", "Secret99", "accountdb"} {
		if !strings.Contains(sql, part) {
			t.Fatalf("missing %q in %s", part, sql)
		}
	}
}

func TestGameAccountSQLMuEmu(t *testing.T) {
	info, sql, err := gameAccountSQL("muemu", "testando", "Secret99", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if info.Login != "testando" {
		t.Fatalf("login %s", info.Login)
	}
	for _, part := range []string{"data.\"Account\"", "data.\"ItemStorage\"", "testando", "$2b$", "ON CONFLICT"} {
		if !strings.Contains(sql, part) {
			t.Fatalf("missing %q in %s", part, sql)
		}
	}
}

func TestGameAccountSQLOpenMU(t *testing.T) {
	info, sql, err := gameAccountSQL("openmu", "testando", "Secret99", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if info.Login != "testando" {
		t.Fatalf("login %s", info.Login)
	}
	for _, part := range []string{"data.\"Account\"", "data.\"ItemStorage\"", "testando", "$2b$", "ON CONFLICT"} {
		if !strings.Contains(sql, part) {
			t.Fatalf("missing %q in %s", part, sql)
		}
	}
}

func TestGameAccountSQLRakion(t *testing.T) {
	info, sql, err := gameAccountSQL("rakion", "testando", "Secret99", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if info.Login != "testando" {
		t.Fatalf("login %s", info.Login)
	}
	for _, part := range []string{"rakion.user", "secret99", "usergameinfo"} {
		if !strings.Contains(sql, part) {
			t.Fatalf("missing %q in %s", part, sql)
		}
	}
}
