package service

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"encoding/base64"
	"io"
	"strings"
	"testing"
)

// aesECBDecrypt mirrors what the Rakion client does to read config.xfs.
func aesECBDecrypt(t *testing.T, b64ct string, key []byte) string {
	t.Helper()
	ct, err := base64.StdEncoding.DecodeString(b64ct)
	if err != nil {
		t.Fatalf("b64 decode: %v", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes: %v", err)
	}
	pt := make([]byte, len(ct))
	for i := 0; i < len(ct); i += aes.BlockSize {
		block.Decrypt(pt[i:i+aes.BlockSize], ct[i:i+aes.BlockSize])
	}
	return string(pt)
}

func field(t *testing.T, text, key string) string {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, key+"@") {
			return strings.TrimRight(strings.TrimPrefix(line, key+"@"), " \r\x00")
		}
	}
	return ""
}

func TestBuildRakionConfigXfsRoundTrip(t *testing.T) {
	const host = "rakion.luxview.cloud"

	raw, err := BuildRakionConfigXfs(host)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(raw), "\r\n"), "\r\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 CRLF-separated lines, got %d: %q", len(lines), raw)
	}

	// line2 ([sec]) decrypts with the fixed key and carries the random key.
	sec := aesECBDecrypt(t, lines[1], []byte(rakionFixedKey))
	if !strings.Contains(sec, "[sec]") {
		t.Fatalf("line2 did not decrypt to [sec] block: %q", sec)
	}
	rkB64 := field(t, sec, "key")
	rk, err := base64.StdEncoding.DecodeString(rkB64)
	if err != nil || len(rk) != 16 {
		t.Fatalf("bad random key: %q (len %d, err %v)", rkB64, len(rk), err)
	}

	// line1 ([server]) decrypts with the random key and carries the host.
	server := aesECBDecrypt(t, lines[0], rk)
	if !strings.Contains(server, "[server]") {
		t.Fatalf("line1 did not decrypt to [server] block: %q", server)
	}
	if got := field(t, server, "ip"); got != host {
		t.Fatalf("ip@ = %q, want %q", got, host)
	}
}

func TestIsRakionConfigEntry(t *testing.T) {
	cases := map[string]bool{
		"config.xfs":         true,
		"Bin/config.xfs":     true,
		"Bin\\config.xfs":    true,
		"CONFIG.XFS":         true,
		"Bin/DataSetup.xfs":  false,
		"config.xfs.bak":     false,
		"Scripts/config.ini": false,
	}
	for name, want := range cases {
		if got := isRakionConfigEntry(name); got != want {
			t.Errorf("isRakionConfigEntry(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestWriteRakionClientZipReplacesConfig(t *testing.T) {
	// Build a tiny base zip with a placeholder config.xfs + an untouched file.
	var baseBuf bytes.Buffer
	zw := zip.NewWriter(&baseBuf)
	for name, content := range map[string]string{
		"Bin/config.xfs": "OLD-PLACEHOLDER",
		"config.xfs":     "OLD-PLACEHOLDER",
		"readme.txt":     "keep me",
	} {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(content))
	}
	zw.Close()

	var out bytes.Buffer
	if err := WriteRakionClientZip(bytes.NewReader(baseBuf.Bytes()), int64(baseBuf.Len()), &out, RakionClientOptions{AuthHost: "rakion.luxview.cloud"}); err != nil {
		t.Fatalf("write: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	seen := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		seen[f.Name] = string(b)
	}
	if seen["readme.txt"] != "keep me" {
		t.Errorf("readme.txt altered: %q", seen["readme.txt"])
	}
	for _, n := range []string{"Bin/config.xfs", "config.xfs"} {
		if seen[n] == "OLD-PLACEHOLDER" || seen[n] == "" {
			t.Errorf("%s was not regenerated: %q", n, seen[n])
		}
		if !strings.Contains(seen[n], "\r\n") {
			t.Errorf("%s not a config.xfs payload: %q", n, seen[n])
		}
	}
}
