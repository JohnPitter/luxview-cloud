package service

import "github.com/luxview/engine/internal/model"

const metin2TemplateID = "metin2"

// metin2Template describes the single-container legacy image used by the
// platform. The legacy client authenticates on 11000 and enters channel 1 on
// 13001; the remaining channel ports are published for future channels and
// the 13099 mark service is kept available for the client handshake.
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
		SupportsQuery: false,
		DefaultVolumes: []model.GameVolume{
			{MountPath: "/var/lib/mysql"},
		},
		ConfigFields: []model.ConfigFieldDef{
			{Key: "METIN_DB_USER", Label: "Usuário do banco do jogo", Type: "text", Placeholder: "user", Section: "Servidor"},
			{Key: "METIN_DB_PASSWORD", Label: "Senha do banco do jogo", Type: "password", Placeholder: "pw", Section: "Servidor"},
			{Key: "METIN_SERVER_NAME", Label: "Nome exibido no cliente", Type: "text", Placeholder: "OpenMetin", Section: "Servidor"},
			{Key: "LUXVIEW_LISTED", Label: "Exibir no launcher LuxView", Type: "select", Options: sel("true", "Sim", "false", "Não"), Section: "Launcher"},
		},
	}
}
