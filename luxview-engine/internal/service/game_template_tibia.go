package service

import "github.com/luxview/engine/internal/model"

const tibiaTemplateID = "tibia"

// tibiaTemplate describes the single-container Canary image. O container roda
// MariaDB + login-server HTTP (porta 8088, usada pelo client OTClient) + o
// servidor Canary (login TCP 7171, jogo 7172, status 7173). O client OTClient
// é servido a partir do armazenamento global com o init.lua injetado apontando
// para http://<server-ip>:8088/login.
func tibiaTemplate() model.GameTemplate {
	return model.GameTemplate{
		ID:               tibiaTemplateID,
		DisplayName:      "Tibia",
		Description:      "Aventure-se em um mundo de fantasia medieval: escolha sua vocação (cavaleiro, paladino, druida ou mago) e enfrente monstros, explore masmorras e complete quests épicas.",
		Protocol:         "tcp",
		DefaultGamePort:  7172,
		DefaultQueryPort: 7171,
		DefaultExtraPorts: []model.ExtraPort{
			{Port: 7173, Protocol: "tcp", Label: "Status"},
			{Port: 8088, Protocol: "tcp", Label: "Login HTTP (OTClient)"},
		},
		DefaultImage:  "luxview-cloud-tibia:latest",
		SupportsQuery: false,
		DefaultVolumes: []model.GameVolume{
			{MountPath: "/var/lib/mysql"},
			{MountPath: "/canary/data-otservbr-global/world"},
		},
		ConfigFields: []model.ConfigFieldDef{
			{Key: "TIBIA_SERVER_NAME", Label: "Nome do Servidor", Type: "text", Placeholder: "LuxView Tibia", Section: "Servidor"},
			{Key: "TIBIA_MAX_PLAYERS", Label: "Máx. Jogadores (0 = sem limite)", Type: "number", Placeholder: "0", Section: "Servidor"},
			// Taxas
			{Key: "TIBIA_RATE_EXP", Label: "Taxa de Experiência", Type: "number", Placeholder: "20", Section: "Taxas"},
			{Key: "TIBIA_RATE_SKILL", Label: "Taxa de Skill", Type: "number", Placeholder: "20", Section: "Taxas"},
			{Key: "TIBIA_RATE_MAGIC", Label: "Taxa de Magia", Type: "number", Placeholder: "20", Section: "Taxas"},
			{Key: "TIBIA_RATE_LOOT", Label: "Taxa de Loot", Type: "number", Placeholder: "5", Section: "Taxas"},
			{Key: "TIBIA_RATE_SPAWN", Label: "Taxa de Spawn", Type: "number", Placeholder: "2", Section: "Taxas"},
			// Gameplay
			{Key: "TIBIA_DEATH_LOSE_PERCENT", Label: "Perda em morte (%)", Type: "number", Placeholder: "0", Section: "Gameplay"},
			{Key: "TIBIA_FREE_PREMIUM", Label: "Premium gratuito", Type: "select", Options: sel("true", "Sim", "false", "Não"), Section: "Gameplay"},
			// Launcher
			{Key: model.GameClientGlobalFileField, Label: "Arquivo do client no armazenamento global", Type: model.ConfigFieldTypeGlobalFile, Placeholder: "tibia-assets/tibia-client-base.zip", Section: "Launcher"},
			{Key: "LUXVIEW_LISTED", Label: "Exibir no launcher LuxView", Type: "select", Options: sel("true", "Sim", "false", "Não"), Section: "Launcher"},
			// Avançado
			{Key: "TIBIA_DB_PASSWORD", Label: "Senha do banco interno", Type: "password", Placeholder: "canary", Section: "Avançado"},
		},
	}
}
