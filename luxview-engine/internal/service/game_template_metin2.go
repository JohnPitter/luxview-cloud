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
			{Key: "METIN_CORE_COUNT", Label: "Canais ch1 (cores)", Type: "number", Placeholder: "1", Section: "Servidor"},
			{Key: "METIN_SERVER_NAME", Label: "Nome exibido no cliente", Type: "text", Placeholder: "OpenMetin", Section: "Servidor"},
			{Key: model.GameClientGlobalFileField, Label: "Arquivo do client no armazenamento global", Type: model.ConfigFieldTypeGlobalFile, Placeholder: "metin2-legacy-assets/metin2-legacy-client-base.zip", Section: "Launcher"},
			{Key: "LUXVIEW_LISTED", Label: "Exibir no launcher LuxView", Type: "select", Options: sel("true", "Sim", "false", "Não"), Section: "Launcher"},
		},
	}
}
