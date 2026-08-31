package service

import (
	"fmt"
	"strings"

	"github.com/luxview/engine/internal/model"
)

// OpenMUAdminPanelPort is the Blazor admin panel inside the OpenMU container.
// Host publish stays on 127.0.0.1; the engine reaches it over game-net.
const OpenMUAdminPanelPort = 18080

func isAdminExtraPort(templateID string, ep model.ExtraPort) bool {
	if templateID == "openmu" && ep.Port == OpenMUAdminPanelPort {
		return true
	}
	label := strings.ToLower(ep.Label)
	return strings.Contains(label, "admin") || strings.Contains(label, "painel")
}

// AdminPanelURL is the docker-network origin of a game's loopback admin UI.
func AdminPanelURL(subdomain string, cfg *model.GameServerConfig) (string, bool) {
	if cfg == nil || strings.TrimSpace(subdomain) == "" {
		return "", false
	}
	port := 0
	for _, ep := range cfg.ExtraPorts {
		if isAdminExtraPort(cfg.TemplateID, ep) {
			port = ep.Port
			break
		}
	}
	if port == 0 && cfg.TemplateID == "openmu" {
		port = OpenMUAdminPanelPort
	}
	if port == 0 {
		return "", false
	}
	return fmt.Sprintf("http://%s:%d", ContainerName(subdomain), port), true
}
