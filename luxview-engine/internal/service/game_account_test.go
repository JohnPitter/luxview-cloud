package service

import (
	"strings"
	"testing"
)

func TestGameAccountSQLTibia(t *testing.T) {
	info, sql, err := gameAccountSQL("tibia", "Joao", "123456")
	if err != nil {
		t.Fatal(err)
	}
	if info.Login != "joao@luxviewot.com" {
		t.Fatalf("login %s", info.Login)
	}
	for _, part := range []string{"canary.accounts", "canary.players", "Knight Sample", "Joao", "7c4a8d09ca3762af61e59520943dc26494f8941b", "joao@luxviewot.com"} {
		if !strings.Contains(sql, part) {
			t.Fatalf("missing %q in %s", part, sql)
		}
	}
}

func TestGameAccountSQLMetin(t *testing.T) {
	_, sql, err := gameAccountSQL("metin2", "testando", "123456")
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"account.account", "*6BB4837EB74329105EE4568DDA7DC67ED2CA2AD9", "testando"} {
		if !strings.Contains(sql, part) {
			t.Fatalf("missing %q in %s", part, sql)
		}
	}
}

func TestGameAccountSQLRakion(t *testing.T) {
	info, sql, err := gameAccountSQL("rakion", "testando", "Secret99")
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
