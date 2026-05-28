package service

import "github.com/luxview/engine/internal/model"

func muemuTemplate() model.GameTemplate {
	seasons := sel(
		"Season6Kor", "Season 6 (KOR)",
		"Season9Eng", "Season 9 (ENG)",
		"Season12Kor", "Season 12 (KOR)",
		"Season16Kor", "Season 16 (KOR)",
		"Season17Kor", "Season 17 (KOR)",
	)

	return model.GameTemplate{
		ID:               "muemu",
		DisplayName:      "MU Online (Season 6–17 - MuEmu)",
		Description:      "Servidor de MU Online multi-season via MuEmu (open source). Suporta Season 6 a 17.",
		Protocol:         "tcp",
		DefaultGamePort:  44405,
		DefaultQueryPort: 55901,
		DefaultImage:     "luxview-cloud-muemu:latest",
		SupportsQuery:    false,
		DefaultVolumes: []model.GameVolume{
			{MountPath: "/muemu-data"},
		},
		ConfigFields: []model.ConfigFieldDef{
			// Servidor
			{Key: "MUEMU_SERVER_NAME", Label: "Nome do Servidor", Type: "text", Placeholder: "MU Online Server", Section: "Servidor"},
			{Key: "MUEMU_SEASON", Label: "Season", Type: "select", Section: "Servidor", Options: seasons},
			{Key: "MUEMU_LANGUAGE", Label: "Idioma", Type: "select", Section: "Servidor", Options: sel("en", "English", "es", "Español", "pt", "Português")},
			{Key: "MUEMU_AUTO_REGISTER", Label: "Auto-registro de contas", Type: "select", Section: "Servidor", Options: sel("true", "Sim", "false", "Não")},
			// Taxas
			{Key: "MUEMU_EXP_RATE", Label: "Taxa de Experiência", Type: "number", Placeholder: "9000", Section: "Taxas"},
			{Key: "MUEMU_DROP_RATE", Label: "Taxa de Drop", Type: "number", Placeholder: "60", Section: "Taxas"},
			{Key: "MUEMU_ZEN_RATE", Label: "Taxa de Zen (Gold)", Type: "number", Placeholder: "10", Section: "Taxas"},
			{Key: "MUEMU_GOLD_EXP", Label: "Experiência Bônus (Gold)", Type: "number", Placeholder: "0", Section: "Taxas"},
			{Key: "MUEMU_MAX_PARTY_LEVEL_DIFF", Label: "Diferença máx. nível em Party", Type: "number", Placeholder: "400", Section: "Taxas"},
			// Cliente
			{Key: "MUEMU_CLIENT_VERSION", Label: "Versão do Cliente", Type: "text", Placeholder: "10525", Section: "Cliente"},
			{Key: "MUEMU_CLIENT_SERIAL", Label: "Serial do Cliente", Type: "text", Placeholder: "fughy683dfu7teqg", Section: "Cliente"},
			// Avançado
			{Key: "MYSQL_ROOT_PASSWORD", Label: "Senha MySQL (interno)", Type: "password", Placeholder: "muemu", Section: "Avançado"},
		},
	}
}
