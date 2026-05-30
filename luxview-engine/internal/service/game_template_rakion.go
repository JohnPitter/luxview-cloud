package service

import "github.com/luxview/engine/internal/model"

// rakionTemplate defines the Rakion (SoftNyx) private server template.
//
// Architecture (single self-contained image running under Wine):
//   - BrokenServer (broker)      : TCP 40706  — first endpoint the client hits
//   - RakionWorldServ (world)    : TCP 40708 + UDP 40709 — gameplay
//   - PHP auth web + admin panel : HTTP 80    — routed via Traefik subdomain (plain HTTP)
//
// The broker/world are raw TCP/UDP (players connect to VPS_IP:40706). The auth
// web (WebPort) is published to the app's AssignedPort and exposed at
// "<subdomain>.<domain>" via Traefik over HTTPS (Let's Encrypt). The client's
// config.xfs points at that subdomain; the launcher login is an HTTP GET, and
// the platform's :80->:443 redirect preserves GET+query, so it reaches HTTPS.
func rakionTemplate() model.GameTemplate {
	return model.GameTemplate{
		ID:               "rakion",
		DisplayName:      "Rakion (SoftNyx v258)",
		Description:      "Servidor privado de Rakion v258 (broker + world sob Wine) com auth web e painel admin PHP.",
		Protocol:         "tcp",
		DefaultGamePort:  40706, // BrokenServer (broker) — cliente conecta aqui primeiro
		DefaultQueryPort: 40708, // RakionWorldServ (world) TCP
		DefaultExtraPorts: []model.ExtraPort{
			{Port: 40708, Protocol: "udp", Label: "World UDP1"}, // gameplay UDP (== world TCP port)
			{Port: 40709, Protocol: "udp", Label: "World UDP2"},
			{Port: 41016, Protocol: "udp", Label: "World UDP3"}, // csauth/extra world UDP
		},
		WebPort:       80, // auth web + painel admin -> roteado por subdomínio (HTTP puro)
		DefaultImage:  "luxview-cloud-rakion:latest",
		SupportsQuery: false, // sem A2S; status = container rodando
		DefaultVolumes: []model.GameVolume{
			{MountPath: "/var/lib/mysql"}, // persiste contas/personagens entre restarts
		},
		ConfigFields: []model.ConfigFieldDef{
			// Avançado — contas são criadas pelo painel admin (web)
			{Key: "RAKION_ADMIN_PASS", Label: "Senha do Painel Admin", Type: "password", Placeholder: "admin123", Section: "Avançado"},
			{Key: "MYSQL_ROOT_PASSWORD", Label: "Senha MySQL (interno)", Type: "password", Placeholder: "123456", Section: "Avançado"},
		},
	}
}
