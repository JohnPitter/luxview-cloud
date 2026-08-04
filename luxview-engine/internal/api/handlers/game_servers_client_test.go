package handlers

import (
	"testing"

	"github.com/luxview/engine/internal/model"
)

func TestGameClientDownloadURLForConfiguredClientTemplates(t *testing.T) {
	const appID = "8f18612d-8cb3-4b0e-b67d-94d26b1ce53f"

	for _, template := range []string{"openmu", "rakion", "metin2"} {
		if got := gameClientDownloadURL(appID, template); got != "/api/apps/"+appID+"/game-client/download" {
			t.Fatalf("%s client url = %q", template, got)
		}
	}

	if got := gameClientDownloadURL(appID, "vrising"); got != "" {
		t.Fatalf("vrising client url = %q", got)
	}
}

func TestStaticGameServerStatusUsesAppStatusWhenTemplateDoesNotSupportQuery(t *testing.T) {
	runningApp := &model.App{Status: model.AppStatusRunning}
	stoppedApp := &model.App{Status: model.AppStatusStopped}
	template := &model.GameTemplate{SupportsQuery: false}

	if got := staticGameServerStatus(runningApp, template); got == nil || !got.Running {
		t.Fatalf("running app static status = %#v", got)
	}
	if got := staticGameServerStatus(stoppedApp, template); got == nil || got.Running {
		t.Fatalf("stopped app static status = %#v", got)
	}
	if got := staticGameServerStatus(runningApp, &model.GameTemplate{SupportsQuery: true}); got != nil {
		t.Fatalf("queryable template static status = %#v", got)
	}
}
