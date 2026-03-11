package service

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
	"github.com/mssola/useragent"
)

// TraefikLogEntry represents one JSON line from Traefik access log.
// Headers configured with "keep" appear as flat keys: request_User-Agent, request_Referer.
type TraefikLogEntry struct {
	ClientHost       string `json:"ClientHost"`
	RequestHost      string `json:"RequestHost"`
	RequestPath      string `json:"RequestPath"`
	RequestMethod    string `json:"RequestMethod"`
	DownstreamStatus int    `json:"DownstreamStatus"`
	Duration         int64  `json:"Duration"` // nanoseconds
	StartUTC         string `json:"StartUTC"`
	UserAgent        string `json:"request_User-Agent"`
	Referer          string `json:"request_Referer"`
}

// LogParser converts Traefik log entries into Pageview models.
type LogParser struct {
	geoip         *GeoIP
	domain        string
	subdomainApps map[string]uuid.UUID
}

func NewLogParser(geoip *GeoIP, domain string) *LogParser {
	return &LogParser{
		geoip:         geoip,
		domain:        domain,
		subdomainApps: make(map[string]uuid.UUID),
	}
}

// UpdateAppCache refreshes the subdomain → app_id mapping.
func (lp *LogParser) UpdateAppCache(apps map[string]uuid.UUID) {
	lp.subdomainApps = apps
}

// ParseLine parses a single JSON log line into a Pageview. Returns nil if the line should be skipped.
func (lp *LogParser) ParseLine(line []byte) *model.Pageview {
	var entry TraefikLogEntry
	if err := json.Unmarshal(line, &entry); err != nil {
		return nil
	}

	// Filter: only GET requests (pageviews)
	if entry.RequestMethod != "GET" {
		return nil
	}

	// Filter: skip non-success responses (allow 404 as it's still a visit)
	if entry.DownstreamStatus < 200 || (entry.DownstreamStatus >= 400 && entry.DownstreamStatus != 404) {
		return nil
	}

	// Filter: skip redirects
	if entry.DownstreamStatus == 301 || entry.DownstreamStatus == 302 {
		return nil
	}

	// Filter: skip static assets
	if isStaticAsset(entry.RequestPath) {
		return nil
	}

	// Filter: skip internal paths
	if isInternalPath(entry.RequestPath) {
		return nil
	}

	// Parse User-Agent
	ua := useragent.New(entry.UserAgent)

	// Filter: skip bots
	if ua.Bot() {
		return nil
	}

	// Filter: skip DevTools emulated devices (not real users)
	if isEmulatedDevice(entry.UserAgent) {
		return nil
	}

	// Resolve app_id from host
	var appID *uuid.UUID
	host := strings.ToLower(entry.RequestHost)
	if host != lp.domain && strings.HasSuffix(host, "."+lp.domain) {
		subdomain := strings.TrimSuffix(host, "."+lp.domain)
		if id, ok := lp.subdomainApps[subdomain]; ok {
			appID = &id
		} else {
			return nil // unknown subdomain, skip
		}
	}
	// If host == domain, appID stays nil (platform)

	// Parse timestamp
	ts, err := time.Parse(time.RFC3339Nano, entry.StartUTC)
	if err != nil {
		ts, err = time.Parse("2006-01-02T15:04:05Z", entry.StartUTC)
		if err != nil {
			return nil
		}
	}

	// GeoIP lookup
	geo := lp.geoip.Lookup(entry.ClientHost)

	// IP hash for privacy (LGPD compliant)
	ipHash := fmt.Sprintf("%x", sha256.Sum256([]byte(entry.ClientHost)))

	// Browser and OS
	browserName, browserVer := ua.Browser()
	osName := ua.OS()

	// Device type
	deviceType := "desktop"
	if ua.Mobile() {
		deviceType = "mobile"
	}

	// Referer: skip self-referrals
	referer := entry.Referer
	if referer != "" {
		if u, err := url.Parse(referer); err == nil {
			if strings.HasSuffix(u.Host, lp.domain) {
				referer = ""
			}
		}
	}

	// Duration: Traefik reports in nanoseconds
	responseMs := int(entry.Duration / 1_000_000)

	return &model.Pageview{
		AppID:      appID,
		Timestamp:  ts,
		Path:       entry.RequestPath,
		Method:     entry.RequestMethod,
		StatusCode: entry.DownstreamStatus,
		IPHash:     ipHash,
		Country:    geo.Country,
		City:       geo.City,
		Region:     geo.Region,
		Browser:    browserName,
		BrowserVer: browserVer,
		OS:         osName,
		DeviceType: deviceType,
		Referer:    referer,
		ResponseMs: responseMs,
	}
}

func isStaticAsset(path string) bool {
	suffixes := []string{".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".map", ".webp"}
	lower := strings.ToLower(path)
	for _, s := range suffixes {
		if strings.HasSuffix(lower, s) {
			return true
		}
	}
	prefixes := []string{"/assets/", "/static/", "/_next/static/", "/fonts/"}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

func isInternalPath(path string) bool {
	internals := []string{"/health", "/api/internal/", "/favicon.", "/robots.txt", "/sitemap.xml", "/manifest.json"}
	lower := strings.ToLower(path)
	for _, p := range internals {
		if strings.HasPrefix(lower, p) || lower == p {
			return true
		}
	}
	return false
}

// isEmulatedDevice detects Chrome DevTools "Toggle Device Toolbar" emulated devices.
// These send fake mobile UAs from a desktop browser and pollute analytics.
func isEmulatedDevice(ua string) bool {
	emulatedSignatures := []string{
		"Nexus 5 Build/MRA58N",  // Chrome DevTools default mobile
		"Nexus 5X Build/",       // DevTools preset
		"Nexus 6P Build/",       // DevTools preset
		"Pixel 2 Build/",        // DevTools preset
		"Pixel 2 XL Build/",     // DevTools preset
		"Pixel 3 Build/",        // DevTools preset
	}
	for _, sig := range emulatedSignatures {
		if strings.Contains(ua, sig) {
			return true
		}
	}
	return false
}
