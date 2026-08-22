package service

import "github.com/luxview/engine/internal/model"

const pristonTemplateID = "priston"

func pristonTemplate() model.GameTemplate {
	return model.GameTemplate{
		ID:              pristonTemplateID,
		DisplayName:     "Priston Tale (4420 / OpenPriston)",
		Description:     "Servidor Priston Tale clássico (versão 4420) com drops e spawns alinhados ao mapa original: Ricarten, Pillai, G1–G3 e F1–F4.",
		Protocol:        "tcp",
		DefaultGamePort: 10012,
		DefaultExtraPorts: []model.ExtraPort{
			{Port: 10013, Protocol: "tcp", Label: "Clan"},
			{Port: 5080, Protocol: "tcp", Label: "Health"},
		},
		DefaultImage:  "luxview-cloud-priston:latest",
		DefaultCPU:    "1.0",
		DefaultMemory: "2g",
		SupportsQuery: false,
		DefaultVolumes: []model.GameVolume{
			{MountPath: "/data/state"},
			{
				MountPath: "/client",
				HostPath:  "/data/luxview/storage/_global/priston-assets/client",
			},
		},
		ConfigFields: []model.ConfigFieldDef{
			{Key: "PRISTON_SERVER_NAME", Label: "Nome do Servidor", Type: "text", Placeholder: "LuxView", Section: "Servidor"},

			{Key: "PRISTON_RATE_EXP", Label: "Experiência (×)", Type: "number", Placeholder: "5", Section: "Taxas", Hint: "Oficial EPT = 1. Mid-rate clássico BR: 5×. A wiki aplica penalidade extra se o mob estiver ±11 levels do char."},
			{Key: "PRISTON_RATE_GOLD", Label: "Gold (×)", Type: "number", Placeholder: "3", Section: "Taxas"},
			{Key: "PRISTON_RATE_DROP", Label: "Drop (×)", Type: "number", Placeholder: "2", Section: "Taxas", Hint: "Sheltoms, armas e sets. Manter abaixo do EXP evita inflação."},

			{Key: "PRISTON_MAX_MOBS", Label: "Mobs visíveis (spots)", Type: "number", Placeholder: "32", Section: "Gameplay", Hint: "Quantos monstros o client vê ao mesmo tempo. Oficial ~32; subir enche os spots (G1–F4)."},
			{Key: "PRISTON_SPAWN_BATCH", Label: "Lote de spawn", Type: "number", Placeholder: "4", Section: "Gameplay", Hint: "Quantos mobs nascem por tick. Maior = spots reenchem mais rápido."},
			{Key: "PRISTON_SPAWN_PROTECTION", Label: "Proteção ao nascer (s)", Type: "number", Placeholder: "10", Section: "Gameplay", Hint: "Segundos de imunidade depois do teleport/respawn (oficial ~10s)."},

			{Key: model.GameClientGlobalFileField, Label: "Arquivo do client no armazenamento global", Type: model.ConfigFieldTypeGlobalFile, Placeholder: "priston-assets/priston-4420-base.zip", Section: "Launcher"},
			{Key: "LUXVIEW_LISTED", Label: "Exibir no launcher LuxView", Type: "select", Options: sel("true", "Sim", "false", "Não"), Section: "Launcher"},
		},
	}
}
