package main

import "testing"

func TestShouldDownloadClientBase(t *testing.T) {
	if !shouldDownloadClientBase(false, "", "abc") {
		t.Fatal("first install must download the base zip")
	}
	if shouldDownloadClientBase(true, "", "abc") {
		t.Fatal("legacy install without a stamp must not re-download the whole zip")
	}
	if shouldDownloadClientBase(true, "abc", "abc") {
		t.Fatal("matching base hash must use overlay patch only")
	}
	if !shouldDownloadClientBase(true, "old", "new") {
		t.Fatal("replaced base zip must download again")
	}
	if !shouldDownloadClientBase(true, "abc", "") {
		t.Fatal("missing catalog base hash must fall back to a full fetch")
	}
}

func TestUsesSplitClient(t *testing.T) {
	if usesSplitClient(GameCard{DownloadURL: "https://x/zip"}) {
		t.Fatal("full-zip-only catalog must keep the legacy download")
	}
	if !usesSplitClient(GameCard{
		BaseURL:   "https://x/base",
		PatchURL:  "https://x/patch",
		BaseHash:  "abc",
	}) {
		t.Fatal("catalog with base+patch must use delta updates")
	}
}

func TestSanitizeCacheKey(t *testing.T) {
	if got := sanitizeCacheKey(`metin2/../abc def`); got != "metin2abcdef" {
		t.Fatalf("sanitize = %q", got)
	}
}
