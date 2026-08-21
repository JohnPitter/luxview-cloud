package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// luxviewBaseURL is the production platform origin, stamped at release via
// -ldflags "-X main.luxviewBaseURL=https://…". Never a VPS IP: game endpoints
// come from the catalog (server_ip / auth_host). Local and CI override with
// LUXVIEW_BASE_URL.
var luxviewBaseURL = ""

var loadLauncherEnvOnce sync.Once

func baseURL() string {
	if v := strings.TrimRight(os.Getenv("LUXVIEW_BASE_URL"), "/"); v != "" {
		return v
	}
	return strings.TrimRight(luxviewBaseURL, "/")
}

func platformOrigin() (string, error) {
	origin := baseURL()
	if origin == "" {
		return "", fmt.Errorf("defina LUXVIEW_BASE_URL com a origem da plataforma (veja .env.example)")
	}
	return origin, nil
}

func loadLauncherDotEnv() {
	loadLauncherEnvOnce.Do(func() {
		for _, path := range launcherEnvPaths() {
			applyDotEnvFile(path)
		}
	})
}

func launcherEnvPaths() []string {
	var paths []string
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, ".env"))
	}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), ".env"))
	}
	return paths
}

// applyDotEnvFile sets KEY=VALUE from a .env without overriding the process
// environment. Existing OS vars always win.
func applyDotEnvFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}
		if os.Getenv(key) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}
}
