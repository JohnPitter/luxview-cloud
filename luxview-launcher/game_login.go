package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type gameLogin struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

func gameLoginPath(appID string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "luxview-launcher", "game-login-"+appID+".json"), nil
}

func saveGameLogin(appID, user, pass string) error {
	path, err := gameLoginPath(appID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(gameLogin{User: user, Pass: pass})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func loadGameLogin(appID string) (gameLogin, error) {
	var creds gameLogin
	path, err := gameLoginPath(appID)
	if err != nil {
		return creds, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return creds, err
	}
	err = json.Unmarshal(data, &creds)
	return creds, err
}
