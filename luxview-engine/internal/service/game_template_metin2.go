package service

import "github.com/luxview/engine/internal/model"

const metin2TemplateID = "metin2"

func metin2Template() model.GameTemplate {
	return model.GameTemplate{
		ID:               metin2TemplateID,
		DisplayName:      "Metin2 Legacy",
		Description:      "Servidor legado de Metin2 já validado com o cliente atual e suporte a Português (Brasil).",
		Protocol:         "tcp",
		DefaultGamePort:  11000,
		DefaultQueryPort: 13001,
		DefaultExtraPorts: []model.ExtraPort{
			{Port: 13002, Protocol: "tcp", Label: "Canal 1 - Core 2"},
			{Port: 13003, Protocol: "tcp", Label: "Canal 1 - Core 3"},
			{Port: 13004, Protocol: "tcp", Label: "Canal 1 - Core 4"},
			{Port: 13099, Protocol: "tcp", Label: "Game 99"},
		},
		DefaultImage:  "luxview-cloud-metin2-legacy:latest",
		DefaultCPU:    "2.0",
		DefaultMemory: "3g",
		DBService:     model.ServiceMySQL,
		SupportsQuery: false,
		ConfigFields: []model.ConfigFieldDef{
			{Key: "METIN_CORE_COUNT", Label: "Canais ch1 (cores)", Type: "number", Placeholder: "1", Section: "Servidor", Hint: "1 core = 1 canal de jogo. Mais cores = mais mapas simultâneos no mesmo channel."},
			{Key: "METIN_SERVER_NAME", Label: "Nome exibido no cliente", Type: "text", Placeholder: "OpenMetin", Section: "Servidor"},
			{Key: "METIN_MAX_LEVEL", Label: "Nível máximo", Type: "number", Placeholder: "99", Section: "Servidor", Hint: "Oficial clássico capava em 99; muitos PSs BR usam 120."},

			{Key: "METIN_RATE_EXP", Label: "Experiência (×)", Type: "number", Placeholder: "10", Section: "Taxas", Hint: "1 = oficial Gameforge. Mid-rate BR típico: 10–50×. Aplicado como bônus de império permanente."},
			{Key: "METIN_RATE_YANG", Label: "Yang (×)", Type: "number", Placeholder: "10", Section: "Taxas", Hint: "Ouro dropado por mobs. Oficial = 1."},
			{Key: "METIN_RATE_DROP", Label: "Drop de itens (×)", Type: "number", Placeholder: "5", Section: "Taxas", Hint: "Pedras Metin, upgrades e loot de mob. Costuma ser menor que o EXP."},

			{Key: "METIN_GAME99", Label: "Canal 99 (mapa extra)", Type: "select", Options: sel("false", "Não", "true", "Sim"), Section: "Gameplay", Hint: "Sobe o core game99 (porta 13099) para mapas especiais / GM."},

			{Key: model.GameClientGlobalFileField, Label: "Arquivo do client no armazenamento global", Type: model.ConfigFieldTypeGlobalFile, Placeholder: "metin2-legacy-assets/metin2-legacy-client-base.zip", Section: "Launcher"},
			{Key: "LUXVIEW_LISTED", Label: "Exibir no launcher LuxView", Type: "select", Options: sel("true", "Sim", "false", "Não"), Section: "Launcher"},
		},
	}
}
