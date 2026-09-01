package service

import "github.com/luxview/engine/internal/model"

func openmuTemplate() model.GameTemplate {
	return model.GameTemplate{
		ID:               "openmu",
		DisplayName:      "MU Online (LuxView)",
		Description:      "Season 99d, 2 e 6 pt 3 — Marketplace, Auto Battle e client LuxView. Template principal do MU na plataforma.",
		Protocol:         "tcp",
		DefaultGamePort:  44405,
		DefaultQueryPort: 55901,
		DefaultExtraPorts: []model.ExtraPort{
			{Port: 55980, Protocol: "tcp", Label: "ChatServer"},
			{Port: 18080, Protocol: "tcp", Label: "Painel Admin"},
			{Port: 55902, Protocol: "tcp", Label: "GameServer 0b"},
			{Port: 55903, Protocol: "tcp", Label: "GameServer 1"},
			{Port: 55904, Protocol: "tcp", Label: "GameServer 1b"},
			{Port: 55905, Protocol: "tcp", Label: "GameServer 2"},
			{Port: 55906, Protocol: "tcp", Label: "GameServer 2b"},
		},
		DefaultImage:  "luxview-cloud-openmu:latest",
		DefaultCPU:    "1.0",
		DefaultMemory: "2g",
		DBService:     model.ServicePostgres,
		SupportsQuery: false,
		DefaultVolumes: []model.GameVolume{
			{MountPath: "/openmu-data"},
		},
		ConfigFields: []model.ConfigFieldDef{
			{Key: "OPENMU_ADMIN_USER", Label: "Usuário Admin (painel web)", Type: "text", Placeholder: "admin", Section: "Servidor"},
			{Key: "OPENMU_ADMIN_PASS", Label: "Senha Admin (painel web)", Type: "password", Placeholder: "openmu", Section: "Servidor"},
			{Key: "OPENMU_SERVER_NAME", Label: "Nome do Servidor", Type: "text", Placeholder: "MU Online Server", Section: "Servidor"},
			{Key: "OPENMU_DESCRIPTION", Label: "Descrição", Type: "text", Placeholder: "Servidor LuxView MU", Section: "Servidor"},
			{Key: "OPENMU_MAX_CONNECTIONS", Label: "Máx. Conexões por GameServer", Type: "number", Placeholder: "1000", Section: "Servidor"},

			{Key: "OPENMU_EXP_RATE", Label: "Experiência (×)", Type: "number", Placeholder: "10", Section: "Taxas", Hint: "1 = Season 6 oficial. Mid-rate BR: 10–50×. Grava em GameConfiguration.ExperienceRate."},
			{Key: "OPENMU_MASTER_EXP_RATE", Label: "Master EXP (×)", Type: "number", Placeholder: "10", Section: "Taxas", Hint: "EXP depois do level 400 (master). Oficial = 1."},
			{Key: "OPENMU_ZEN_RATE", Label: "Zen (×)", Type: "number", Placeholder: "5", Section: "Taxas", Hint: "Gold dropado. Oficial = 1."},
			{Key: "OPENMU_EXCELLENT_DELTA", Label: "Delta de Excellent drop", Type: "number", Placeholder: "0", Section: "Taxas", Hint: "Níveis a mais que o mob precisa ter vs DropLevel do item excellent. 0 = padrão OpenMU."},

			{Key: "OPENMU_MAX_LEVEL", Label: "Nível máximo", Type: "number", Placeholder: "400", Section: "Gameplay", Hint: "Cap Season 6 antes do master. Oficial = 400."},
			{Key: "OPENMU_MAX_MASTER_LEVEL", Label: "Master level máximo", Type: "number", Placeholder: "400", Section: "Gameplay"},
			{Key: "OPENMU_MAX_PARTY", Label: "Tamanho máx. da party", Type: "number", Placeholder: "5", Section: "Gameplay"},
			{Key: "OPENMU_POINTS_PER_LEVEL", Label: "Pontos por Nível", Type: "number", Placeholder: "5", Section: "Gameplay", Hint: "Status points ao upar. Oficial S6 = 5 (DL/MG variam no painel)."},
			{Key: "OPENMU_PK_ENABLED", Label: "PvP (PK) Habilitado", Type: "select", Section: "Gameplay", Options: sel("true", "Sim", "false", "Não")},

			{Key: model.GameClientGlobalFileField, Label: "Arquivo do client no armazenamento global", Type: model.ConfigFieldTypeGlobalFile, Placeholder: "openmu-assets/openmu-s6-base.zip", Section: "Launcher"},
			{Key: "LUXVIEW_LISTED", Label: "Exibir no launcher LuxView", Type: "select", Options: sel("true", "Sim", "false", "Não"), Section: "Launcher"},
			{Key: "POSTGRES_PASSWORD", Label: "Senha PostgreSQL (interno)", Type: "password", Placeholder: "openmu", Section: "Avançado"},
		},
	}
}
