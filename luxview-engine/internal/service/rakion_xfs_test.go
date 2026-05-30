package service

import (
	"encoding/base64"
	"strings"
	"testing"
)

// realNyxLauncherEnc is the actual NyxLauncherEnc.xfs from the Rakion client,
// holding the launcher's fetch/auto-download URLs (hard-coded to the dev host).
const realNyxLauncherEnc = "fwAAAAsBgHMAAAAAeJyL9qus8EkszUvOSC2K5eUKLcqJd0stSc6wzSgpKbDS1ze0NNIzNLPQM9Qz1U8DSUBIvYKMAl4uzwJbJHleroD8ohJbEwNzAzNeLl6uaPfSvKT80rwUks0F6g1KzM7MzyNHZ3h+TlpJamIuyXoB5JtP9j14nItwCzZiAAJGBgioZ4DwQjJSFSJS81KLM4uNFNIyc1IViiuLS1JzFcpSi4oz8/MUjPQM9AwYGACD7Q6wJgAAeJyLj/errPBJLM1Lzkgt0vP082SgLWABYkYg5gYS1UAaAPJaBso="

func TestRewriteRakionLauncherEncRoundTrip(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(realNyxLauncherEnc)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	// Sanity: parse the original and confirm it carries the dev host.
	_, _, _, files, err := parseXFS2(data)
	if err != nil {
		t.Fatalf("parse original: %v", err)
	}
	if len(files) != 1 || !strings.Contains(string(files[0].content), "Url_Fetch=http://192.168.1.5/fetch/fetch.php") {
		t.Fatalf("unexpected original content: %q", files[0].content)
	}

	out, err := rewriteRakionLauncherEnc(data, "rakion.luxview.cloud", "187.77.227.65")
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	// Re-parse the rewritten container and verify the hosts were swapped.
	_, _, _, files2, err := parseXFS2(out)
	if err != nil {
		t.Fatalf("parse rewritten: %v", err)
	}
	got := string(files2[0].content)
	if strings.Contains(got, "192.168.1.5") {
		t.Errorf("dev host still present:\n%s", got)
	}
	if !strings.Contains(got, "Url_Fetch=http://rakion.luxview.cloud/fetch/fetch.php") {
		t.Errorf("fetch URL not rewritten:\n%s", got)
	}
	if !strings.Contains(got, "Ip=187.77.227.65") {
		t.Errorf("broker Ip not rewritten:\n%s", got)
	}
	// The filename entry must survive the repack.
	if name := string(files2[0].name[:]); !strings.HasPrefix(name, "__NyxLauncher.INI") {
		t.Errorf("filename lost: %q", name)
	}
}
