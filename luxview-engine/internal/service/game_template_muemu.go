package service

import "github.com/luxview/engine/internal/model"

const muemuTemplateID = "muemu"

func muemuTemplate() model.GameTemplate {
	// Yomalex MuEmu: Season0Kor/Season3Kor/Season6Kor share the old packet
	// encode, so a hybrid 97D + Season 2 client can sit on one ConnectServer
	// and pick GS 0 (97D) or GS 99 (Season 2+). Later seasons need their own client.
	seasons := sel(
		"Classic99", "97D + Season 2+ (mesmo client, GS 99)",
		"Season0Kor", "97D + 99 (clássico)",
		"Season3Kor", "Season 2–3",
		"Season6Kor", "Season 6 (KOR)",
		"Season9Eng", "Season 9 (ENG)",
		"Season12Kor", "Season 12 (KOR)",
		"Season16Kor", "Season 16 (KOR)",
		"Season17Kor", "Season 17 (KOR)",
	)

	return model.GameTemplate{
		ID:               muemuTemplateID,
		DisplayName:      "MU Online legado (97D / Season 2+ / S6–17 - MuEmu)",
		Description:      "Template legado via MuEmu. Preferir OpenMU (Season 6) para o stack LuxView atual. A edição 97D + Season 2+ sobe o ConnectServer 99 com dois mundos no mesmo client: GS 0 (97D) e GS 99 (Season 2+).",
		Protocol:         "tcp",
		DefaultGamePort:  44405,
		DefaultQueryPort: 55901,
		DefaultExtraPorts: []model.ExtraPort{
			{Port: 55919, Protocol: "tcp", Label: "GameServer 99"},
			{Port: 55980, Protocol: "tcp", Label: "ChatServer"},
		},
		DefaultImage:  "luxview-cloud-muemu:latest",
		DefaultCPU:    "1.0",
		DefaultMemory: "2g",
		SupportsQuery: false,
		DefaultVolumes: []model.GameVolume{
			{MountPath: "/muemu-data"},
		},
		ConfigFields: []model.ConfigFieldDef{
			{Key: "MUEMU_SERVER_NAME", Label: "Nome do Servidor", Type: "text", Placeholder: "MU Online Server", Section: "Servidor"},
			{Key: "MUEMU_SEASON", Label: "Versão", Type: "select", Section: "Servidor", Options: seasons},
			{Key: "MUEMU_LANGUAGE", Label: "Idioma", Type: "select", Section: "Servidor", Options: sel("en", "English", "es", "Español", "pt", "Português")},
			{Key: "MUEMU_AUTO_REGISTER", Label: "Auto-registro de contas", Type: "select", Section: "Servidor", Options: sel("true", "Sim", "false", "Não")},
			{Key: "MUEMU_EXP_RATE", Label: "Experiência (×)", Type: "number", Placeholder: "9000", Section: "Taxas", Hint: "MuEmu usa valor absoluto no Server.xml. 9000 é high-rate clássico; 1–50 é mid/low."},
			{Key: "MUEMU_DROP_RATE", Label: "Drop (×)", Type: "number", Placeholder: "60", Section: "Taxas", Hint: "Chance de item no MonsterSetBase. 60 = high-rate; 1–10 = closer to oficial."},
			{Key: "MUEMU_ZEN_RATE", Label: "Zen (×)", Type: "number", Placeholder: "10", Section: "Taxas"},
			{Key: "MUEMU_GOLD_EXP", Label: "Gold EXP (bônus)", Type: "number", Placeholder: "0", Section: "Taxas", Hint: "EXP extra paga em Zen (eventos). 0 = desligado."},
			{Key: "MUEMU_MAX_PARTY_LEVEL_DIFF", Label: "Diferença máx. de level na party", Type: "number", Placeholder: "400", Section: "Gameplay", Hint: "Oficial ignora party se a diferença de level for grande demais."},
			{Key: "MUEMU_CLIENT_VERSION", Label: "Versão do Cliente", Type: "text", Placeholder: "10203", Section: "Cliente"},
			{Key: "MUEMU_CLIENT_SERIAL", Label: "Serial do Cliente", Type: "text", Placeholder: "fughy683dfu7teqg", Section: "Cliente"},
			{Key: model.GameClientGlobalFileField, Label: "Arquivo do client no armazenamento global", Type: model.ConfigFieldTypeGlobalFile, Placeholder: "muemu-assets/mu-97d-s2-base.zip", Section: "Launcher"},
			{Key: "LUXVIEW_LISTED", Label: "Exibir no launcher LuxView", Type: "select", Options: sel("true", "Sim", "false", "Não"), Section: "Launcher"},
			{Key: "MYSQL_ROOT_PASSWORD", Label: "Senha MySQL (interno)", Type: "password", Placeholder: "muemu", Section: "Avançado"},
		},
	}
}
