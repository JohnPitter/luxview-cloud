package service

import (
	"fmt"
	"strings"

	"github.com/luxview/engine/internal/model"
)

func firstField(fields map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(fields[k]); v != "" {
			return v
		}
	}
	return ""
}

func isLoopbackHost(host string) bool {
	switch host {
	case "", "127.0.0.1", "localhost", "::1":
		return true
	default:
		return false
	}
}

// EmbeddedGameDBCreds describes the MySQL the game is already using.
// Existing production games keep MariaDB inside the container (127.0.0.1);
// new games get mysql-shared via Provision. This does not create a database.
func EmbeddedGameDBCreds(tmpl *model.GameTemplate, cfg *model.GameServerConfig) (string, map[string]string) {
	fields := map[string]string{}
	if cfg != nil && cfg.ConfigFields != nil {
		fields = cfg.ConfigFields
	}

	host := firstField(fields, "MYSQL_HOST", "METIN_MYSQL_HOST")
	port := firstField(fields, "MYSQL_PORT", "METIN_MYSQL_PORT")
	user := firstField(fields, "MYSQL_USER", "METIN_DB_USER")
	pass := firstField(fields, "MYSQL_PASSWORD", "METIN_DB_PASSWORD", "TIBIA_DB_PASSWORD", "MYSQL_ROOT_PASSWORD")
	db := firstField(fields, "MYSQL_DATABASE")
	id := ""
	if tmpl != nil {
		id = tmpl.ID
	}

	switch id {
	case "tibia":
		if host == "" {
			host = "127.0.0.1"
		}
		if port == "" {
			port = "3306"
		}
		if user == "" {
			user = "canary"
		}
		if pass == "" {
			pass = "canary"
		}
		if db == "" {
			db = "canary"
		}
	case "rakion":
		if host == "" {
			host = "127.0.0.1"
		}
		if port == "" {
			port = "3306"
		}
		if user == "" {
			user = "root"
		}
		if pass == "" {
			pass = "123456"
		}
		if db == "" {
			db = "rakion"
		}
	case "metin2":
		if host == "" {
			host = "127.0.0.1"
		}
		if port == "" {
			port = "3306"
		}
		if user == "" {
			user = "user"
		}
		if pass == "" {
			pass = "pw"
		}
		if db == "" {
			db = "account"
		}
	case "muemu":
		if host == "" {
			host = "127.0.0.1"
		}
		if port == "" {
			port = "3306"
		}
		if user == "" {
			user = "root"
		}
		if pass == "" {
			pass = "muemu"
		}
		if db == "" {
			db = "MuOnline"
		}
	default:
		if host == "" {
			host = "127.0.0.1"
		}
		if port == "" {
			port = "3306"
		}
		if db == "" && id != "" {
			db = id
		}
	}

	location := "shared"
	if isLoopbackHost(host) {
		location = "embedded"
	}
	creds := map[string]string{
		"host":     host,
		"port":     port,
		"database": db,
		"username": user,
		"password": pass,
		"url":      fmt.Sprintf("mysql://%s:%s@%s:%s/%s", user, pass, host, port, db),
		"location": location,
	}
	return db, creds
}
