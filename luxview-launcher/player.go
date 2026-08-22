package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

type PlayerSession struct {
	Token        string `json:"token"`
	Username     string `json:"username"`
	CashPoints   int64  `json:"cash_points"`
	RewardPoints int64  `json:"reward_points"`
}

type playerAPISession struct {
	Token  string `json:"token"`
	Player struct {
		Username     string `json:"username"`
		CashPoints   int64  `json:"cash_points"`
		RewardPoints int64  `json:"reward_points"`
	} `json:"player"`
}

func (a *App) PlayerRegister(username, password string) (PlayerSession, error) {
	return a.playerAuth("/api/public/players/register", username, password)
}

func (a *App) PlayerLogin(username, password string) (PlayerSession, error) {
	return a.playerAuth("/api/public/players/login", username, password)
}

func (a *App) PlayerMe() (PlayerSession, error) {
	var out PlayerSession
	origin, err := platformOrigin()
	if err != nil {
		return out, err
	}
	sess, err := loadPlayerSession()
	if err != nil || sess.Token == "" {
		return out, fmt.Errorf("entre na conta LuxView")
	}
	req, err := http.NewRequest(http.MethodGet, origin+"/api/players/me", nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Authorization", "Bearer "+sess.Token)
	resp, err := a.client.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("sessão expirada")
	}
	var me struct {
		Username     string `json:"username"`
		CashPoints   int64  `json:"cash_points"`
		RewardPoints int64  `json:"reward_points"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&me); err != nil {
		return out, err
	}
	out = PlayerSession{Token: sess.Token, Username: me.Username, CashPoints: me.CashPoints, RewardPoints: me.RewardPoints}
	_ = savePlayerSession(out)
	return out, nil
}

func (a *App) PlayerLogout() error {
	_ = clearPlayerSecret()
	path, err := playerSessionPath()
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (a *App) PlayerLink(appID, templateID, nick string) error {
	origin, err := platformOrigin()
	if err != nil {
		return err
	}
	sess, err := loadPlayerSession()
	if err != nil || sess.Token == "" {
		return fmt.Errorf("entre na conta LuxView")
	}
	body, _ := json.Marshal(map[string]string{"app_id": appID, "template_id": templateID, "nick": nick})
	req, err := http.NewRequest(http.MethodPost, origin+"/api/players/links", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+sess.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("%s", stringsTrimError(raw))
	}
	return nil
}

type ShopItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TemplateID  string `json:"template_id"`
	Currency    string `json:"currency"`
	Price       int64  `json:"price"`
	Grant       string `json:"grant"`
	Amount      int64  `json:"amount"`
}

type ShopBuyResult struct {
	Item         ShopItem `json:"item"`
	CashPoints   int64    `json:"cash_points"`
	RewardPoints int64    `json:"reward_points"`
}

func (a *App) ShopCatalog(templateID string) ([]ShopItem, error) {
	origin, err := platformOrigin()
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Get(origin + "/api/public/shop?template=" + url.QueryEscape(templateID))
	if err != nil {
		return nil, fmt.Errorf("não consegui carregar a loja: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("%s", stringsTrimError(raw))
	}
	var items []ShopItem
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&items); err != nil {
		return nil, fmt.Errorf("catálogo inválido")
	}
	if items == nil {
		items = []ShopItem{}
	}
	return items, nil
}

func (a *App) ShopBuy(appID, itemID string) (ShopBuyResult, error) {
	var out ShopBuyResult
	origin, err := platformOrigin()
	if err != nil {
		return out, err
	}
	sess, err := loadPlayerSession()
	if err != nil || sess.Token == "" {
		return out, fmt.Errorf("entre na conta LuxView")
	}
	body, _ := json.Marshal(map[string]string{"app_id": appID, "item_id": itemID})
	req, err := http.NewRequest(http.MethodPost, origin+"/api/players/shop/buy", bytes.NewReader(body))
	if err != nil {
		return out, err
	}
	req.Header.Set("Authorization", "Bearer "+sess.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return out, fmt.Errorf("não consegui concluir a compra: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return out, fmt.Errorf("%s", stringsTrimError(raw))
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("resposta inválida")
	}
	sess.CashPoints = out.CashPoints
	sess.RewardPoints = out.RewardPoints
	_ = savePlayerSession(sess)
	return out, nil
}

func (a *App) playerAuth(path, username, password string) (PlayerSession, error) {
	var out PlayerSession
	origin, err := platformOrigin()
	if err != nil {
		return out, err
	}
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	resp, err := a.client.Post(origin+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return out, fmt.Errorf("não consegui contatar o servidor: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return out, fmt.Errorf("%s", stringsTrimError(raw))
	}
	var sess playerAPISession
	if err := json.Unmarshal(raw, &sess); err != nil {
		return out, fmt.Errorf("resposta inválida")
	}
	out = PlayerSession{
		Token: sess.Token, Username: sess.Player.Username,
		CashPoints: sess.Player.CashPoints, RewardPoints: sess.Player.RewardPoints,
	}
	if err := savePlayerSession(out); err != nil {
		return out, err
	}
	if err := savePlayerSecret(out.Username, password); err != nil {
		return out, err
	}
	return out, nil
}

func stringsTrimError(raw []byte) string {
	var wrapped struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &wrapped) == nil && wrapped.Error != "" {
		return wrapped.Error
	}
	s := string(bytes.TrimSpace(raw))
	if s == "" {
		return "falha na conta LuxView"
	}
	return s
}

func playerSessionPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "luxview-launcher", "player.json"), nil
}

func savePlayerSession(sess PlayerSession) error {
	path, err := playerSessionPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func loadPlayerSession() (PlayerSession, error) {
	var sess PlayerSession
	path, err := playerSessionPath()
	if err != nil {
		return sess, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return sess, err
	}
	err = json.Unmarshal(data, &sess)
	return sess, err
}

type playerSecret struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func playerSecretPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "luxview-launcher", "player-secret.json"), nil
}

func savePlayerSecret(username, password string) error {
	path, err := playerSecretPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(playerSecret{Username: username, Password: password})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func loadPlayerSecret() (playerSecret, error) {
	var sec playerSecret
	path, err := playerSecretPath()
	if err != nil {
		return sec, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return sec, err
	}
	err = json.Unmarshal(data, &sec)
	return sec, err
}

func clearPlayerSecret() error {
	path, err := playerSecretPath()
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (a *App) ensureGameAccount(appID, password, characterName, vocation string) error {
	origin, err := platformOrigin()
	if err != nil {
		return err
	}
	sess, err := loadPlayerSession()
	if err != nil || sess.Token == "" {
		return fmt.Errorf("entre na conta LuxView")
	}
	body, _ := json.Marshal(map[string]string{
		"password":       password,
		"character_name": characterName,
		"vocation":       vocation,
	})
	req, err := http.NewRequest(http.MethodPost, origin+"/api/players/games/"+appID+"/account", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+sess.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("não consegui criar a conta no jogo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("%s", stringsTrimError(raw))
	}
	return nil
}
