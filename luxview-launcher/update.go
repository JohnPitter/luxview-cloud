package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/minio/selfupdate"
)

// UpdateInfo is returned to the frontend so it can show an "update available"
// banner. When Available is false the launcher is already on the latest release.
type UpdateInfo struct {
	Available bool   `json:"available"`
	Current   string `json:"current"`
	Version   string `json:"version"`
	URL       string `json:"url"`
	Notes     string `json:"notes"`
}

// latestRelease mirrors the engine's /api/public/launcher/latest payload.
type latestRelease struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	Notes   string `json:"notes"`
}

// CheckForUpdate asks the platform for the latest published launcher release and
// compares it against the running version. The platform proxies GitHub so we get
// one branded source and don't hit GitHub's rate limit from every client.
func (a *App) CheckForUpdate() (UpdateInfo, error) {
	info := UpdateInfo{Available: false, Current: appVersion}

	resp, err := a.client.Get(baseURL() + "/api/public/launcher/latest")
	if err != nil {
		return info, fmt.Errorf("falha ao checar atualização: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		// No release published yet — not an error from the user's perspective.
		return info, nil
	}
	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("falha ao checar atualização: status %d", resp.StatusCode)
	}

	var rel latestRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return info, fmt.Errorf("resposta de atualização inválida: %w", err)
	}

	info.Version = rel.Version
	info.URL = rel.URL
	info.Notes = rel.Notes
	info.Available = rel.URL != "" && versionLess(appVersion, rel.Version)
	return info, nil
}

// ApplyUpdate downloads the new launcher binary, swaps it in place (selfupdate
// handles the Windows "binary is running" rename), and relaunches the app. The
// launcher runs elevated (requireAdministrator), so writing next to itself works
// even under Program Files.
func (a *App) ApplyUpdate(url string) error {
	if url == "" {
		return fmt.Errorf("URL de atualização vazia")
	}

	resp, err := a.dl.Get(url)
	if err != nil {
		return fmt.Errorf("falha ao baixar atualização: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("falha ao baixar atualização: status %d", resp.StatusCode)
	}

	if err := selfupdate.Apply(resp.Body, selfupdate.Options{}); err != nil {
		// selfupdate already attempts a rollback on failure.
		return fmt.Errorf("falha ao aplicar atualização: %w", err)
	}

	// Relaunch the now-updated binary and exit the current process.
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("atualizado, mas falha ao reiniciar: %w", err)
	}
	cmd := exec.Command(exe)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("atualizado, mas falha ao reiniciar: %w", err)
	}
	os.Exit(0)
	return nil
}

// versionLess reports whether version a is strictly older than b. Versions look
// like "v1.32"; we compare the dotted integer components, so "v1.9" < "v1.10".
// Non-numeric / unparizable input compares as 0 for that component.
func versionLess(a, b string) bool {
	pa := parseVersion(a)
	pb := parseVersion(b)
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			return x < y
		}
	}
	return false
}

func parseVersion(v string) []int {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, _ := strconv.Atoi(strings.TrimSpace(p))
		out = append(out, n)
	}
	return out
}
