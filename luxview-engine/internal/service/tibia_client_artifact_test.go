package service

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func TestBuildTibiaInitLuaInjectsServer(t *testing.T) {
	base := []byte("local x = 1\n[\"http://127.0.0.1:8088/login\"] = { protocol = 1525 }\n")
	got := string(buildTibiaInitLua(base, TibiaClientOptions{
		ServerName: "LuxView Tibia",
		ServerIP:   "187.77.227.65",
		LoginPort:  8088,
	}))
	assertContains(t, got, "187.77.227.65:8088")
	if bytes.Contains([]byte(got), []byte("127.0.0.1:8088")) {
		t.Fatalf("init.lua still contains placeholder: %q", got)
	}
}

func TestWriteTibiaClientZipPatchesInitLua(t *testing.T) {
	var base bytes.Buffer
	baseZip := zip.NewWriter(&base)
	addZipEntry(t, baseZip, "otclient.exe", "fake-exe")
	addZipEntry(t, baseZip, "./init.lua", "[\"http://127.0.0.1:8088/login\"] = { protocol = 1525 }")
	if err := baseZip.Close(); err != nil {
		t.Fatalf("close base zip: %v", err)
	}

	var out bytes.Buffer
	if err := WriteTibiaClientZip(bytes.NewReader(base.Bytes()), int64(base.Len()), &out, TibiaClientOptions{
		ServerName: "LuxView Tibia",
		ServerIP:   "187.77.227.65",
		LoginPort:  8088,
	}); err != nil {
		t.Fatalf("write client zip: %v", err)
	}

	got := readZipEntries(t, out.Bytes())
	if got["otclient.exe"] != "fake-exe" {
		t.Fatalf("otclient.exe content = %q", got["otclient.exe"])
	}
	initLua := got["./init.lua"]
	if initLua == "" {
		initLua = got["init.lua"]
	}
	assertContains(t, initLua, "187.77.227.65:8088")
	if bytes.Contains([]byte(initLua), []byte("127.0.0.1:8088")) {
		t.Fatalf("init.lua still contains placeholder: %q", initLua)
	}
}

func TestWriteTibiaClientPatchOmitsUnpatchedFiles(t *testing.T) {
	var base bytes.Buffer
	baseZip := zip.NewWriter(&base)
	addZipEntry(t, baseZip, "otclient.exe", strings.Repeat("X", 256*1024))
	addZipEntry(t, baseZip, "init.lua", "[\"http://127.0.0.1:8088/login\"] = { protocol = 1525 }")
	if err := baseZip.Close(); err != nil {
		t.Fatalf("close base zip: %v", err)
	}

	var full, patch bytes.Buffer
	opts := TibiaClientOptions{ServerName: "LuxView Tibia", ServerIP: "187.77.227.65", LoginPort: 8088}
	if err := WriteTibiaClientZip(bytes.NewReader(base.Bytes()), int64(base.Len()), &full, opts); err != nil {
		t.Fatalf("full zip: %v", err)
	}
	if err := WriteTibiaClientPatch(bytes.NewReader(base.Bytes()), int64(base.Len()), &patch, opts); err != nil {
		t.Fatalf("patch zip: %v", err)
	}
	if patch.Len() >= full.Len() {
		t.Fatalf("patch %d bytes should be smaller than full %d", patch.Len(), full.Len())
	}
	got := readZipEntries(t, patch.Bytes())
	if _, ok := got["otclient.exe"]; ok {
		t.Fatal("patch must not include the unpatched client binary")
	}
	initLua := got["init.lua"]
	if initLua == "" {
		initLua = got["./init.lua"]
	}
	assertContains(t, initLua, "187.77.227.65:8088")
}
