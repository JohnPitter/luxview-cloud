package service

import "github.com/luxview/engine/internal/model"

// rakionTemplate defines the Rakion (SoftNyx) private server template.
//
// Architecture (single image under Wine):
//   - BrokenServer (broker)      : TCP 40706
//   - RakionWorldServ (world)    : TCP 40708 + UDP 40709
//   - PHP auth web + admin panel : HTTP 80
//
// MySQL lives in mysql-shared. The launcher login is POST (never query).
func rakionTemplate() model.GameTemplate {
	return model.GameTemplate{
		ID:               "rakion",
		DisplayName:      "Rakion (SoftNyx v258)",
		Description:      "Servidor privado de Rakion v258 (broker + world sob Wine) com auth web e painel admin PHP.",
		Protocol:         "tcp",
		DefaultGamePort:  40706,
		DefaultQueryPort: 40708,
		DefaultExtraPorts: []model.ExtraPort{
			{Port: 40708, Protocol: "udp", Label: "World UDP1"},
			{Port: 40709, Protocol: "udp", Label: "World UDP2"},
			{Port: 41016, Protocol: "udp", Label: "World UDP3"},
		},
		WebPort:       80,
		DefaultImage:  "luxview-cloud-rakion:latest",
		DefaultCPU:    "1.0",
		DefaultMemory: "512m",
		DBService:     model.ServiceMySQL,
		SupportsQuery: false,
		ConfigFields: []model.ConfigFieldDef{
			{Key: model.GameClientGlobalFileField, Label: "Arquivo do client no armazenamento global", Type: model.ConfigFieldTypeGlobalFile, Placeholder: "rakion-assets/rakion-client-base.zip", Section: "Launcher"},
			{Key: "LUXVIEW_LISTED", Label: "Exibir no launcher LuxView", Type: "select", Options: sel("true", "Sim", "false", "Não"), Section: "Launcher"},
			{Key: "RAKION_ADMIN_PASS", Label: "Senha do Painel Admin", Type: "password", Placeholder: "admin123", Section: "Avançado"},
		},
	}
}
