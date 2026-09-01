package service

import (
	"fmt"

	"github.com/luxview/engine/internal/model"
)

func openmuTemplate() model.GameTemplate {
	fields := []model.ConfigFieldDef{
		{Key: "OPENMU_ADMIN_USER", Label: "Usuário Admin (painel web)", Type: "text", Placeholder: "admin", Section: "Servidor"},
		{Key: "OPENMU_ADMIN_PASS", Label: "Senha Admin (painel web)", Type: "password", Placeholder: "openmu", Section: "Servidor"},
		{Key: "OPENMU_SERVER_NAME", Label: "Nome do Servidor", Type: "text", Placeholder: "MU Online Server", Section: "Servidor"},
		{Key: "OPENMU_DESCRIPTION", Label: "Descrição", Type: "text", Placeholder: "Servidor LuxView MU", Section: "Servidor"},
		{Key: "OPENMU_MAX_CONNECTIONS", Label: "Máx. Conexões por GameServer", Type: "number", Placeholder: "1000", Section: "Servidor"},

		{Key: "OPENMU_EXP_RATE", Label: "Experiência (×)", Type: "number", Placeholder: "10", Section: "Taxas globais", Hint: "1 = Season 6 oficial. Mid-rate BR: 10–50×. Grava em GameConfiguration.ExperienceRate."},
		{Key: "OPENMU_MASTER_EXP_RATE", Label: "Master EXP (×)", Type: "number", Placeholder: "10", Section: "Taxas globais", Hint: "EXP depois do level 400 (master). Oficial = 1."},
		{Key: "OPENMU_ZEN_RATE", Label: "Zen (×)", Type: "number", Placeholder: "5", Section: "Taxas globais", Hint: "Quantidade de gold dropado. Oficial = 1."},
		{Key: "OPENMU_EXCELLENT_DELTA", Label: "Delta de Excellent drop", Type: "number", Placeholder: "0", Section: "Taxas globais", Hint: "Níveis a mais que o mob precisa ter vs DropLevel do item excellent. 0 = padrão OpenMU."},

		{Key: "OPENMU_MAX_LEVEL", Label: "Nível máximo", Type: "number", Placeholder: "400", Section: "Gameplay", Hint: "Cap Season 6 antes do master. Oficial = 400."},
		{Key: "OPENMU_MAX_MASTER_LEVEL", Label: "Master level máximo", Type: "number", Placeholder: "400", Section: "Gameplay"},
		{Key: "OPENMU_MAX_PARTY", Label: "Tamanho máx. da party", Type: "number", Placeholder: "5", Section: "Gameplay", Hint: "Status points ao upar. Oficial S6 = 5 (DL/MG variam no painel)."},
		{Key: "OPENMU_POINTS_PER_LEVEL", Label: "Pontos por Nível", Type: "number", Placeholder: "5", Section: "Gameplay", Hint: "Status points ao upar. Oficial S6 = 5."},
		{Key: "OPENMU_PK_ENABLED", Label: "PvP (PK) Habilitado", Type: "select", Section: "Gameplay", Options: sel("true", "Sim", "false", "Não")},

		{Key: model.GameClientGlobalFileField, Label: "Arquivo do client no armazenamento global", Type: model.ConfigFieldTypeGlobalFile, Placeholder: "openmu-assets/openmu-s6-base.zip", Section: "Launcher"},
		{Key: "LUXVIEW_LISTED", Label: "Exibir no launcher LuxView", Type: "select", Options: sel("true", "Sim", "false", "Não"), Section: "Launcher"},
		{Key: "POSTGRES_PASSWORD", Label: "Senha PostgreSQL (interno)", Type: "password", Placeholder: "openmu", Section: "Avançado"},
	}

	fields = append(fields, openmuPerServerRateFields(0, "Season 6 Pt 3", openmuServerRateDefaults{"35", "50", "20", "100", "100", "50", "25", "50", "60", "20", "80", "50", "45"})...)
	fields = append(fields, openmuPerServerRateFields(20, "99d", openmuServerRateDefaults{"15", "25", "5", "100", "100", "50", "25", "50", "60", "20", "80", "50", "45"})...)
	fields = append(fields, openmuPerServerRateFields(40, "Season 2", openmuServerRateDefaults{"60", "75", "35", "100", "100", "50", "25", "50", "60", "20", "80", "50", "45"})...)

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
		ConfigFields: fields,
	}
}

type openmuServerRateDefaults struct {
	itemDrop, zenDrop, jewelDrop       string
	mixMult                            string
	bless, soul, soulLuck              string
	life, harmony                      string
	lowerRefine, higherRefine          string
	mixPlus10, mixPlus11               string
}

func openmuPerServerRateFields(serverID int, label string, d openmuServerRateDefaults) []model.ConfigFieldDef {
	p := fmt.Sprintf("OPENMU_S%d_", serverID)
	section := fmt.Sprintf("Taxas — %s", label)
	return []model.ConfigFieldDef{
		{Key: p + "ITEM_DROP", Label: "Drop de item (%)", Type: "number", Placeholder: d.itemDrop, Section: section, Hint: "Chance (0–100) de item de grupo cair. GameServerDefinition.ItemDropRate."},
		{Key: p + "ZEN_DROP", Label: "Drop de zen (%)", Type: "number", Placeholder: d.zenDrop, Section: section, Hint: "Chance (0–100) de zen cair (quantidade usa Zen global ×)."},
		{Key: p + "JEWEL_DROP", Label: "Drop de joia (%)", Type: "number", Placeholder: d.jewelDrop, Section: section, Hint: "Chance (0–100) de joia clássica cair."},
		{Key: p + "MIX_MULT", Label: "Mix Chaos (×)", Type: "number", Placeholder: d.mixMult, Section: section, Hint: "Multiplica a taxa final de sucesso de mixes (asas, fenrir, tickets, etc.). 100 = padrão OpenMU."},
		{Key: p + "BLESS_RATE", Label: "Bless +level (%)", Type: "number", Placeholder: d.bless, Section: section, Hint: "Jewel of Bless: chance de subir +1 level (+0→+6). Oficial = 100."},
		{Key: p + "SOUL_RATE", Label: "Soul +level (%)", Type: "number", Placeholder: d.soul, Section: section, Hint: "Jewel of Soul: chance de subir +1 level (+0→+9). Oficial = 50."},
		{Key: p + "SOUL_LUCK", Label: "Soul bônus Luck (%)", Type: "number", Placeholder: d.soulLuck, Section: section, Hint: "Bônus extra quando o item tem opção Luck. Oficial = +25."},
		{Key: p + "LIFE_RATE", Label: "Life opção (%)", Type: "number", Placeholder: d.life, Section: section, Hint: "Jewel of Life: adicionar/subir opção adicional. Falha remove opção. Oficial = 50."},
		{Key: p + "HARMONY_RATE", Label: "Harmony (%)", Type: "number", Placeholder: d.harmony, Section: section, Hint: "Jewel of Harmony: adicionar opção Harmony. Oficial = 60."},
		{Key: p + "REFINE_LOW", Label: "Refine baixo (%)", Type: "number", Placeholder: d.lowerRefine, Section: section, Hint: "Lower Refine Stone: subir opção Harmony. Oficial = 20."},
		{Key: p + "REFINE_HIGH", Label: "Refine alto (%)", Type: "number", Placeholder: d.higherRefine, Section: section, Hint: "Higher Refine Stone: subir opção Harmony. Oficial = 80."},
		{Key: p + "MIX_PLUS10", Label: "Mix +10 (%)", Type: "number", Placeholder: d.mixPlus10, Section: section, Hint: "Chaos Machine: mix +9→+10. Oficial = 50."},
		{Key: p + "MIX_PLUS11", Label: "Mix +11 (%)", Type: "number", Placeholder: d.mixPlus11, Section: section, Hint: "Chaos Machine: mix +10→+11. Oficial = 45."},
	}
}
