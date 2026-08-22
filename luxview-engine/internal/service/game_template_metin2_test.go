package service

import "testing"

func TestMetin2TemplateRegistered(t *testing.T) {
	tmpl := Template(metin2TemplateID)
	if tmpl == nil {
		t.Fatal("metin2 template not registered in Templates()")
	}
	if tmpl.DefaultGamePort != 11000 || tmpl.DefaultQueryPort != 13001 {
		t.Fatalf("ports = %d/%d, want 11000/13001", tmpl.DefaultGamePort, tmpl.DefaultQueryPort)
	}
	if tmpl.Protocol != "tcp" {
		t.Fatalf("protocol = %q, want tcp", tmpl.Protocol)
	}
	if tmpl.DefaultImage != "luxview-cloud-metin2-legacy:latest" {
		t.Fatalf("image = %q, want luxview-cloud-metin2-legacy:latest", tmpl.DefaultImage)
	}

	ports := map[int]bool{}
	for _, port := range tmpl.DefaultExtraPorts {
		ports[port.Port] = port.Protocol == "tcp"
	}
	for _, port := range []int{13002, 13003, 13004, 13099} {
		if !ports[port] {
			t.Errorf("missing TCP extra port %d", port)
		}
	}

	var listed bool
	var hasExp bool
	for _, field := range tmpl.ConfigFields {
		if field.Key == "LUXVIEW_LISTED" {
			listed = true
		}
		if field.Key == "METIN_RATE_EXP" {
			hasExp = true
			if field.Section != "Taxas" {
				t.Errorf("METIN_RATE_EXP section = %q, want Taxas", field.Section)
			}
		}
	}
	if !listed {
		t.Error("missing LUXVIEW_LISTED launcher opt-in field")
	}
	if !hasExp {
		t.Error("missing METIN_RATE_EXP (wiki rates belong in the dashboard)")
	}
}
