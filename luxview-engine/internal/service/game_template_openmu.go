package service

import "github.com/luxview/engine/internal/model"

func openmuTemplate() model.GameTemplate {
	return model.GameTemplate{
		ID:               "openmu",
		DisplayName:      "MU Online (Season 6 - OpenMU)",
		Description:      "Servidor de MU Online Season 6 Episode 3 via OpenMU (open source).",
		Protocol:         "tcp",
		DefaultGamePort:  44405,
		DefaultQueryPort: 55901,
		DefaultExtraPorts: []model.ExtraPort{
			{Port: 55980, Protocol: "tcp", Label: "ChatServer"},
			{Port: 8080, Protocol: "tcp", Label: "Painel Admin"},
			// OpenMU's default data defines multiple GameServers (Server 0/1/2) on
			// ports 55901-55906. The ConnectServer advertises these endpoints to
			// clients, so they must all be published or players get disconnected
			// right after picking a server from the list. 55901 is the main game
			// port (DefaultQueryPort); 55902-55906 are the rest.
			{Port: 55902, Protocol: "tcp", Label: "GameServer 0b"},
			{Port: 55903, Protocol: "tcp", Label: "GameServer 1"},
			{Port: 55904, Protocol: "tcp", Label: "GameServer 1b"},
			{Port: 55905, Protocol: "tcp", Label: "GameServer 2"},
			{Port: 55906, Protocol: "tcp", Label: "GameServer 2b"},
		},
		DefaultImage:  "luxview-cloud-openmu:latest",
		SupportsQuery: false,
		DefaultVolumes: []model.GameVolume{
			{MountPath: "/openmu-data"},
		},
		ConfigFields: []model.ConfigFieldDef{
			// Servidor
			{Key: "OPENMU_ADMIN_USER", Label: "Usuário Admin (painel web)", Type: "text", Placeholder: "admin", Section: "Servidor"},
			{Key: "OPENMU_ADMIN_PASS", Label: "Senha Admin (painel web)", Type: "password", Placeholder: "openmu", Section: "Servidor"},
			{Key: "OPENMU_SERVER_NAME", Label: "Nome do Servidor", Type: "text", Placeholder: "MU Online Server", Section: "Servidor"},
			{Key: "OPENMU_DESCRIPTION", Label: "Descrição", Type: "text", Placeholder: "Servidor LuxView MU", Section: "Servidor"},
			{Key: "OPENMU_MAX_CONNECTIONS", Label: "Máx. Conexões por GameServer", Type: "number", Placeholder: "1000", Section: "Servidor"},
			// Taxas
			{Key: "OPENMU_EXP_RATE", Label: "Taxa de Experiência", Type: "number", Placeholder: "1.0", Section: "Taxas"},
			{Key: "OPENMU_DROP_RATE", Label: "Taxa de Drop", Type: "number", Placeholder: "1.0", Section: "Taxas"},
			{Key: "OPENMU_ZEN_RATE", Label: "Taxa de Zen (Gold)", Type: "number", Placeholder: "1.0", Section: "Taxas"},
			// Gameplay
			{Key: "OPENMU_MAX_LEVEL", Label: "Nível Máximo", Type: "number", Placeholder: "400", Section: "Gameplay"},
			{Key: "OPENMU_POINTS_PER_LEVEL", Label: "Pontos por Nível", Type: "number", Placeholder: "5", Section: "Gameplay"},
			{Key: "OPENMU_PK_ENABLED", Label: "PvP (PK) Habilitado", Type: "select", Section: "Gameplay", Options: sel("true", "Sim", "false", "Não")},
			// Avançado
			{Key: "POSTGRES_PASSWORD", Label: "Senha PostgreSQL (interno)", Type: "password", Placeholder: "openmu", Section: "Avançado"},
		},
	}
}
