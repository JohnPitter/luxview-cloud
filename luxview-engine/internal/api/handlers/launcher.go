package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// launcherTagPrefix scopes which releases carry the desktop launcher. The repo is
// a monorepo, so other (future) releases must not be mistaken for the launcher.
const launcherTagPrefix = "launcher-"

// launcherCacheTTL bounds how often we hit GitHub's API (60 req/h unauthenticated).
const launcherCacheTTL = 5 * time.Minute

// LauncherRelease is the resolved "latest" launcher build the clients consume.
type LauncherRelease struct {
	Version string `json:"version"` // tag without the "launcher-" prefix (e.g. "v1.32")
	URL     string `json:"url"`     // public asset download URL
	Notes   string `json:"notes"`   // release body / changelog
}

// LauncherHandler resolves the latest launcher release from GitHub and exposes it
// to the public landing page (download redirect) and to the launcher itself
// (auto-update check). Results are cached to respect GitHub's rate limit.
type LauncherHandler struct {
	owner     string
	name      string
	assetName string
	token     string // optional GITHUB_TOKEN to raise the rate limit
	client    *http.Client

	mu       sync.Mutex
	cached   *LauncherRelease
	cachedAt time.Time
}

// NewLauncherHandler builds the handler. repo is "owner/name"; assetName is the
// release asset filename (e.g. "luxview-launcher.exe"). token may be empty.
func NewLauncherHandler(repo, assetName, token string) *LauncherHandler {
	owner, name, _ := strings.Cut(repo, "/")
	return &LauncherHandler{
		owner:     owner,
		name:      name,
		assetName: assetName,
		token:     token,
		client:    &http.Client{Timeout: 15 * time.Second},
	}
}

// ghRelease mirrors the subset of the GitHub Releases API we need.
type ghRelease struct {
	TagName    string `json:"tag_name"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// latest returns the most recent published launcher release, using the cache when
// fresh. Callers must handle a nil result (no release yet).
func (h *LauncherHandler) latest() (*LauncherRelease, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cached != nil && time.Since(h.cachedAt) < launcherCacheTTL {
		return h.cached, nil
	}

	rel, err := h.fetchLatest()
	if err != nil {
		// Serve a stale cache rather than failing if GitHub hiccups.
		if h.cached != nil {
			return h.cached, nil
		}
		return nil, err
	}
	h.cached = rel
	h.cachedAt = time.Now()
	return rel, nil
}

func (h *LauncherHandler) fetchLatest() (*LauncherRelease, error) {
	if h.owner == "" || h.name == "" {
		return nil, fmt.Errorf("launcher release repo not configured")
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=30", h.owner, h.name)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github releases: status %d", resp.StatusCode)
	}

	var releases []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	// Releases come newest-first; take the first published launcher release that
	// has our asset.
	for i := range releases {
		r := releases[i]
		if r.Draft || r.Prerelease || !strings.HasPrefix(r.TagName, launcherTagPrefix) {
			continue
		}
		for _, a := range r.Assets {
			if a.Name == h.assetName {
				return &LauncherRelease{
					Version: strings.TrimPrefix(r.TagName, launcherTagPrefix),
					URL:     a.BrowserDownloadURL,
					Notes:   r.Body,
				}, nil
			}
		}
	}
	return nil, nil
}

// Download (GET /api/public/launcher) redirects to the latest launcher asset so
// the platform owns the public URL while GitHub hosts the bytes.
func (h *LauncherHandler) Download(w http.ResponseWriter, r *http.Request) {
	rel, err := h.latest()
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to resolve latest launcher release")
		return
	}
	if rel == nil {
		writeError(w, http.StatusNotFound, "no launcher release published yet")
		return
	}
	http.Redirect(w, r, rel.URL, http.StatusFound)
}

// Latest (GET /api/public/launcher/latest) returns the latest launcher release as
// JSON, consumed by the launcher's auto-update check.
func (h *LauncherHandler) Latest(w http.ResponseWriter, r *http.Request) {
	rel, err := h.latest()
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to resolve latest launcher release")
		return
	}
	if rel == nil {
		writeError(w, http.StatusNotFound, "no launcher release published yet")
		return
	}
	writeJSON(w, http.StatusOK, rel)
}
