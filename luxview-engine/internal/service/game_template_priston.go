package service

import "github.com/luxview/engine/internal/model"

const pristonTemplateID = "priston"

func pristonTemplate() model.GameTemplate {
	return model.GameTemplate{
		ID:              pristonTemplateID,
		DisplayName:     "LuxView Priston",
		Description:     "Servidor nativo Priston Tale Brasil 4220: Ricarten, Pillai, G1–G3 e o conteúdo clássico da wiki. Wine + MSSQL, não OpenPriston/Reloaded.",
		Protocol:        "tcp",
		DefaultGamePort: 10012,
		DefaultExtraPorts: []model.ExtraPort{
			{Port: 10013, Protocol: "tcp", Label: "Clan"},
			{Port: 5080, Protocol: "tcp", Label: "Health"},
		},
		DefaultImage:  "luxview-cloud-priston:latest",
		DefaultCPU:    "2.0",
		DefaultMemory: "4g",
		PidsLimit:     4096,
		SupportsQuery: false,
		DefaultVolumes: []model.GameVolume{
			{MountPath: "/data/state"},
			{
				MountPath: "/server",
				HostPath:  "/data/luxview/storage/_global/priston-assets/server-4220",
			},
		},
		ConfigFields: []model.ConfigFieldDef{
			{Key: "PRISTON_SERVER_NAME", Label: "Nome do Servidor", Type: "text", Placeholder: "LuxView", Section: "Servidor"},
			{Key: "PRISTON_MSSQL_HOST", Label: "Host MSSQL", Type: "text", Placeholder: "luxview-mssql", Section: "Servidor", Hint: "Container SQL Server na rede luxview-net."},
			{Key: "PRISTON_MSSQL_PASSWORD", Label: "Senha SA do MSSQL", Type: "password", Placeholder: "", Section: "Servidor"},

			{Key: "PRISTON_RATE_EXP", Label: "Experiência (×)", Type: "number", Placeholder: "5", Section: "Taxas", Hint: "Aplicado em hotuk.ini só se o evento nativo *EVENT_EXPUP for entendido; caso contrário o 4220 fica no rate oficial."},
			{Key: "PRISTON_RATE_GOLD", Label: "Gold (×)", Type: "number", Placeholder: "3", Section: "Taxas"},
			{Key: "PRISTON_RATE_DROP", Label: "Drop (×)", Type: "number", Placeholder: "2", Section: "Taxas"},

			{Key: model.GameClientGlobalFileField, Label: "Arquivo do client no armazenamento global", Type: model.ConfigFieldTypeGlobalFile, Placeholder: "priston-assets/priston-4220-base.zip", Section: "Launcher"},
			{Key: "LUXVIEW_LISTED", Label: "Exibir no launcher LuxView", Type: "select", Options: sel("true", "Sim", "false", "Não"), Section: "Launcher"},
		},
	}
}
