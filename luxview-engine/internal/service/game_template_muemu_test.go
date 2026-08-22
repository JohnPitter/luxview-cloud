package service

import "testing"

func TestMuEmuTemplateClassic99(t *testing.T) {
	tmpl := Template(muemuTemplateID)
	if tmpl == nil {
		t.Fatal("muemu template not registered in Templates()")
	}
	if tmpl.DefaultGamePort != 44405 || tmpl.DefaultQueryPort != 55901 {
		t.Fatalf("ports = %d/%d, want 44405/55901", tmpl.DefaultGamePort, tmpl.DefaultQueryPort)
	}

	var hasGS99, hasChat, hasClassic, hasListed bool
	for _, port := range tmpl.DefaultExtraPorts {
		if port.Port == 55919 && port.Protocol == "tcp" {
			hasGS99 = true
		}
		if port.Port == 55980 && port.Protocol == "tcp" {
			hasChat = true
		}
	}
	if !hasGS99 {
		t.Error("missing TCP extra port 55919 (GameServer 99)")
	}
	if !hasChat {
		t.Error("missing TCP extra port 55980 (ChatServer)")
	}

	for _, field := range tmpl.ConfigFields {
		if field.Key == "LUXVIEW_LISTED" {
			hasListed = true
		}
		if field.Key != "MUEMU_SEASON" {
			continue
		}
		for _, opt := range field.Options {
			if opt.Value == "Classic99" {
				hasClassic = true
			}
		}
	}
	if !hasClassic {
		t.Error("missing Classic99 season (97D + Season 2+ same client)")
	}
	if !hasListed {
		t.Error("missing LUXVIEW_LISTED launcher opt-in field")
	}
}
