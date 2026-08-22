package service

import "testing"

func TestPristonTemplateRegistered(t *testing.T) {
	tmpl := Template("priston")
	if tmpl == nil {
		t.Fatal("priston template not registered in Templates()")
	}
	if tmpl.DefaultGamePort != 10012 {
		t.Errorf("DefaultGamePort = %d, want 10012", tmpl.DefaultGamePort)
	}
	if tmpl.Protocol != "tcp" {
		t.Errorf("Protocol = %q, want tcp", tmpl.Protocol)
	}
	if tmpl.SupportsQuery {
		t.Error("SupportsQuery = true, want false")
	}
	if tmpl.DefaultImage != "luxview-cloud-priston:latest" {
		t.Errorf("DefaultImage = %q, want luxview-cloud-priston:latest", tmpl.DefaultImage)
	}

	var clientBind string
	for _, v := range tmpl.DefaultVolumes {
		if v.MountPath == "/client" {
			clientBind = v.HostPath
		}
	}
	if clientBind != "/data/luxview/storage/_global/priston-assets/client" {
		t.Errorf("client HostPath = %q, want global priston-assets/client bind", clientBind)
	}

	want := map[int]string{10013: "tcp", 5080: "tcp"}
	for _, ep := range tmpl.DefaultExtraPorts {
		delete(want, ep.Port)
	}
	for port, proto := range want {
		t.Errorf("missing extra port %d/%s", port, proto)
	}

	keys := map[string]string{}
	for _, field := range tmpl.ConfigFields {
		keys[field.Key] = field.Section
	}
	if keys["PRISTON_RATE_EXP"] != "Taxas" {
		t.Error("missing PRISTON_RATE_EXP in Taxas")
	}
	if keys["PRISTON_MAX_MOBS"] != "Gameplay" {
		t.Error("missing PRISTON_MAX_MOBS (spots) in Gameplay")
	}
}
