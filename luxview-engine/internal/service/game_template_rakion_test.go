package service

import (
	"testing"

	"github.com/luxview/engine/internal/model"
)

func TestRakionTemplateRegistered(t *testing.T) {
	tmpl := Template("rakion")
	if tmpl == nil {
		t.Fatal("rakion template not registered in Templates()")
	}
	if tmpl.WebPort != 80 {
		t.Errorf("WebPort = %d, want 80 (auth web routed via Traefik)", tmpl.WebPort)
	}
	if tmpl.DefaultGamePort != 40706 {
		t.Errorf("DefaultGamePort = %d, want 40706 (broker)", tmpl.DefaultGamePort)
	}
	if tmpl.DefaultQueryPort != 40708 {
		t.Errorf("DefaultQueryPort = %d, want 40708 (world)", tmpl.DefaultQueryPort)
	}
	if tmpl.SupportsQuery {
		t.Error("SupportsQuery = true, want false (Rakion has no A2S query)")
	}
	if tmpl.Protocol != "tcp" {
		t.Errorf("Protocol = %q, want tcp", tmpl.Protocol)
	}

	// World UDP must be published as an extra port.
	var hasUDP bool
	for _, ep := range tmpl.DefaultExtraPorts {
		if ep.Port == 40709 && ep.Protocol == "udp" {
			hasUDP = true
		}
	}
	if !hasUDP {
		t.Error("missing extra port 40709/udp (World UDP)")
	}

	// Must persist accounts in mysql-shared, not a volume inside the game cgroup.
	if tmpl.DBService != model.ServiceMySQL {
		t.Errorf("DBService = %q, want mysql", tmpl.DBService)
	}
	for _, v := range tmpl.DefaultVolumes {
		if v.MountPath == "/var/lib/mysql" {
			t.Error("mysql volume must not stay on the game container")
		}
	}
}
