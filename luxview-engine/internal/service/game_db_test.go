package service

import (
	"testing"

	"github.com/luxview/engine/internal/model"
)

func TestEmbeddedGameDBCredsTibiaDefaults(t *testing.T) {
	tmpl := tibiaTemplate()
	db, creds := EmbeddedGameDBCreds(&tmpl, &model.GameServerConfig{})
	if db != "canary" {
		t.Fatalf("db = %q, want canary", db)
	}
	if creds["host"] != "127.0.0.1" || creds["username"] != "canary" || creds["location"] != "embedded" {
		t.Fatalf("creds = %#v", creds)
	}
}

func TestEmbeddedGameDBCredsUsesConfigFields(t *testing.T) {
	tmpl := rakionTemplate()
	db, creds := EmbeddedGameDBCreds(&tmpl, &model.GameServerConfig{
		ConfigFields: map[string]string{
			"MYSQL_HOST":     "luxview-mysql-shared",
			"MYSQL_PORT":     "3306",
			"MYSQL_USER":     "urakion",
			"MYSQL_PASSWORD": "secret",
			"MYSQL_DATABASE": "rakion",
		},
	})
	if db != "rakion" {
		t.Fatalf("db = %q", db)
	}
	if creds["host"] != "luxview-mysql-shared" || creds["location"] != "shared" {
		t.Fatalf("creds = %#v", creds)
	}
	if creds["username"] != "urakion" || creds["password"] != "secret" {
		t.Fatalf("creds = %#v", creds)
	}
}

func TestEmbeddedGameDBCredsMetin2Defaults(t *testing.T) {
	tmpl := metin2Template()
	db, creds := EmbeddedGameDBCreds(&tmpl, nil)
	if db != "account" || creds["username"] != "user" || creds["password"] != "pw" {
		t.Fatalf("db=%q creds=%#v", db, creds)
	}
}
