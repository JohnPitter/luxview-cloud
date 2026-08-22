package service

import (
	"testing"

	"github.com/docker/docker/api/types/mount"
	"github.com/luxview/engine/internal/model"
)

func TestBuildMountsBindsHostPathForPristonClient(t *testing.T) {
	mounts := buildMounts("pt", &model.GameServerConfig{
		Volumes: []model.GameVolume{
			{Name: "luxview-game-pt-data-state", MountPath: "/data/state"},
			{
				Name:      "luxview-game-pt-server",
				MountPath: "/server",
				HostPath:  "/data/luxview/storage/_global/priston-assets/server-4220",
			},
		},
	})
	if len(mounts) != 2 {
		t.Fatalf("len(mounts) = %d, want 2", len(mounts))
	}
	if mounts[0].Type != mount.TypeVolume || mounts[0].Target != "/data/state" {
		t.Errorf("state mount = %+v", mounts[0])
	}
	if mounts[1].Type != mount.TypeBind || mounts[1].Source != "/data/luxview/storage/_global/priston-assets/server-4220" {
		t.Errorf("server mount = %+v", mounts[1])
	}
	if mounts[1].ReadOnly {
		t.Error("server bind must be writable (hotuk.ini, logs)")
	}
}
