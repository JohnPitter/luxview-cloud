package service

import "github.com/luxview/engine/internal/model"

const tibiaTemplateID = "tibia"

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
		DefaultCPU:    "0.5",
		DefaultMemory: "2g",
		DBService:     model.ServiceMySQL,
		SupportsQuery: false,
		DefaultVolumes: []model.GameVolume{
			{MountPath: "/canary/data-otservbr-global/world"},
		},
		ConfigFields: []model.ConfigFieldDef{
			{Key: "TIBIA_SERVER_NAME", Label: "Nome do Servidor", Type: "text", Placeholder: "LuxView Tibia", Section: "Servidor"},
			{Key: "TIBIA_MOTD", Label: "MOTD (mensagem do dia)", Type: "text", Placeholder: "Bem-vindo ao Tibia LuxView!", Section: "Servidor"},
			{Key: "TIBIA_MAX_PLAYERS", Label: "Máx. Jogadores (0 = sem limite)", Type: "number", Placeholder: "0", Section: "Servidor"},
			{Key: "TIBIA_WORLD_TYPE", Label: "Tipo de mundo", Type: "select", Section: "Servidor", Options: sel(
				"pvp", "PvP (oficial)",
				"no-pvp", "Sem PvP",
				"pvp-enforced", "PvP forçado",
			), Hint: "No Tibia oficial o mundo padrão é PvP. Sem PvP impede skull; PvP forçado é war-server."},
			{Key: "TIBIA_ONE_PLAYER_PER_ACCOUNT", Label: "Um personagem online por conta", Type: "select", Options: sel("true", "Sim", "false", "Não"), Section: "Servidor"},

			{Key: "TIBIA_RATE_EXP", Label: "Experiência (×)", Type: "number", Placeholder: "20", Section: "Taxas", Hint: "1 = Tibia oficial. Servidores BR mid-rate costumam usar 8–30×. Stages.lua é desligado para este valor valer."},
			{Key: "TIBIA_RATE_SKILL", Label: "Skills (×)", Type: "number", Placeholder: "20", Section: "Taxas", Hint: "Espada, dist, club, axe e shielding. Oficial = 1."},
			{Key: "TIBIA_RATE_MAGIC", Label: "Magic Level (×)", Type: "number", Placeholder: "20", Section: "Taxas", Hint: "ML de magos/druidas. Oficial = 1."},
			{Key: "TIBIA_RATE_LOOT", Label: "Loot (×)", Type: "number", Placeholder: "5", Section: "Taxas", Hint: "Chance de drop nos corpses. Oficial = 1; 3–8× é o usual em OT mid-rate."},
			{Key: "TIBIA_RATE_SPAWN", Label: "Spawn / spots (×)", Type: "number", Placeholder: "2", Section: "Taxas", Hint: "Velocidade de respawn dos monstros no mapa. 1 = oficial; 2× deixa os spots mais cheios."},
			{Key: "TIBIA_RATE_MONSTER_HEALTH", Label: "HP dos monstros (×)", Type: "number", Placeholder: "1", Section: "Taxas"},
			{Key: "TIBIA_RATE_MONSTER_ATTACK", Label: "Ataque dos monstros (×)", Type: "number", Placeholder: "1", Section: "Taxas"},
			{Key: "TIBIA_RATE_MONSTER_DEFENSE", Label: "Defesa dos monstros (×)", Type: "number", Placeholder: "1", Section: "Taxas"},

			{Key: "TIBIA_PROTECTION_LEVEL", Label: "Protection level", Type: "number", Placeholder: "7", Section: "Gameplay", Hint: "Até este level o jogador não toma PK (oficial = 7, Island of Destiny)."},
			{Key: "TIBIA_PZ_LOCK_SECONDS", Label: "PZ-lock após ataque (s)", Type: "number", Placeholder: "60", Section: "Gameplay", Hint: "Tempo bloqueado na protection zone depois de atacar. Oficial = 60s."},
			{Key: "TIBIA_DEATH_LOSE_PERCENT", Label: "Perda em morte (%)", Type: "number", Placeholder: "0", Section: "Gameplay", Hint: "0 = sem perda; -1 = fórmula oficial; 10 = fórmula antiga."},
			{Key: "TIBIA_FREE_PREMIUM", Label: "Premium gratuito", Type: "select", Options: sel("true", "Sim", "false", "Não"), Section: "Gameplay"},
			{Key: "TIBIA_AUTO_LOOT", Label: "Auto-loot", Type: "select", Options: sel("false", "Não", "true", "Sim"), Section: "Gameplay"},
			{Key: "TIBIA_STAMINA_SYSTEM", Label: "Sistema de stamina", Type: "select", Options: sel("true", "Sim", "false", "Não"), Section: "Gameplay", Hint: "Oficial usa stamina: abaixo de 14h o EXP cai pela metade."},
			{Key: "TIBIA_HOUSE_RENT", Label: "Aluguel de house", Type: "select", Section: "Gameplay", Options: sel(
				"never", "Nunca",
				"weekly", "Semanal",
				"monthly", "Mensal",
				"yearly", "Anual",
			)},
			{Key: "TIBIA_KICK_IDLE_MINUTES", Label: "Kick por inatividade (min)", Type: "number", Placeholder: "15", Section: "Gameplay"},

			{Key: model.GameClientGlobalFileField, Label: "Arquivo do client no armazenamento global", Type: model.ConfigFieldTypeGlobalFile, Placeholder: "tibia-assets/tibia-client-base.zip", Section: "Launcher"},
			{Key: "LUXVIEW_LISTED", Label: "Exibir no launcher LuxView", Type: "select", Options: sel("true", "Sim", "false", "Não"), Section: "Launcher"},
		},
	}
}
