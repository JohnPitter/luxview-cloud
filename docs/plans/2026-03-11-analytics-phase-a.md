# Platform Analytics — Phase A Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Collect pageview analytics from Traefik access logs, enrich with GeoIP and User-Agent parsing, store in DB.

**Architecture:** Traefik writes JSON access logs to a shared volume. An Analytics Worker in the Engine reads new log lines every 60s, parses them, resolves app by subdomain, enriches with MaxMind GeoLite2 (country/city) and UA parsing (browser/OS/device), then bulk inserts into a `pageviews` table.

**Tech Stack:** Go, pgx, maxminddb-golang, mssola/useragent, Traefik v3 JSON access logs

---

### Task 1: Traefik Access Log Configuration

**Files:**
- Modify: `traefik/traefik.yml` (production)
- Modify: `traefik/traefik.dev.yml` (development)
- Modify: `docker-compose.yml` (add shared volume)

**Step 1: Update traefik.yml to write access logs to file with extra headers**

In `traefik/traefik.yml`, update the accessLog section:

```yaml
accessLog:
  filePath: /var/log/traefik/access.json
  format: json
  filters:
    statusCodes:
      - "200-599"
  fields:
    headers:
      defaultMode: drop
      names:
        User-Agent: keep
        Referer: keep
```

**Step 2: Update traefik.dev.yml similarly**

```yaml
accessLog:
  filePath: /var/log/traefik/access.json
  format: json
  fields:
    headers:
      defaultMode: drop
      names:
        User-Agent: keep
        Referer: keep
```

**Step 3: Add shared volume in docker-compose.yml**

Add `traefik-logs` volume:

```yaml
volumes:
  # ... existing volumes ...
  traefik-logs:
```

Mount in both traefik and engine services:

```yaml
  traefik:
    volumes:
      # ... existing volumes ...
      - traefik-logs:/var/log/traefik

  engine:
    volumes:
      # ... existing volumes ...
      - traefik-logs:/var/log/traefik:ro
```

Add `TRAEFIK_LOG_PATH` env var to engine:

```yaml
  engine:
    environment:
      # ... existing vars ...
      - TRAEFIK_LOG_PATH=/var/log/traefik/access.json
```

**Step 4: Commit**

```bash
git add traefik/traefik.yml traefik/traefik.dev.yml docker-compose.yml
git commit -m "feat(analytics): configure Traefik access logs to shared volume with UA/Referer headers"
```

---

### Task 2: Database Tables

**Files:**
- Modify: `luxview-engine/internal/repository/db.go` (add migrations at end of slice)

**Step 1: Add pageviews table migration**

Append to the `migrations` slice in `db.go`:

```go
`CREATE TABLE IF NOT EXISTS pageviews (
    id          BIGSERIAL PRIMARY KEY,
    app_id      UUID REFERENCES apps(id) ON DELETE CASCADE,
    timestamp   TIMESTAMPTZ NOT NULL,
    path        VARCHAR(2048) NOT NULL,
    method      VARCHAR(10) NOT NULL DEFAULT 'GET',
    status_code SMALLINT NOT NULL,
    ip_hash     VARCHAR(64) NOT NULL,
    country     VARCHAR(2),
    city        VARCHAR(128),
    region      VARCHAR(128),
    browser     VARCHAR(64),
    browser_ver VARCHAR(32),
    os          VARCHAR(64),
    device_type VARCHAR(16),
    referer     VARCHAR(2048),
    response_ms INTEGER
)`,

`CREATE INDEX IF NOT EXISTS idx_pv_app_ts ON pageviews(app_id, timestamp DESC)`,
`CREATE INDEX IF NOT EXISTS idx_pv_app_ip ON pageviews(app_id, ip_hash, timestamp)`,
`CREATE INDEX IF NOT EXISTS idx_pv_ts ON pageviews(timestamp DESC)`,
```

**Step 2: Add pageview_aggregations table migration**

Append to the `migrations` slice:

```go
`CREATE TABLE IF NOT EXISTS pageview_aggregations (
    id              BIGSERIAL PRIMARY KEY,
    app_id          UUID REFERENCES apps(id) ON DELETE CASCADE,
    bucket          TIMESTAMPTZ NOT NULL,
    granularity     VARCHAR(4) NOT NULL,
    path            VARCHAR(2048),
    views           INTEGER NOT NULL DEFAULT 0,
    visitors        INTEGER NOT NULL DEFAULT 0,
    bounces         INTEGER NOT NULL DEFAULT 0,
    avg_duration_ms INTEGER,
    country         VARCHAR(2),
    browser         VARCHAR(64),
    os              VARCHAR(64),
    device_type     VARCHAR(16),
    referer_domain  VARCHAR(256)
)`,

`CREATE UNIQUE INDEX IF NOT EXISTS idx_pva_unique ON pageview_aggregations(
    app_id, bucket, granularity,
    COALESCE(path, ''), COALESCE(country, ''), COALESCE(browser, ''),
    COALESCE(os, ''), COALESCE(device_type, ''), COALESCE(referer_domain, ''))`,

`CREATE INDEX IF NOT EXISTS idx_pva_app_bucket ON pageview_aggregations(app_id, bucket DESC, granularity)`,
```

**Step 3: Verify build**

```bash
cd luxview-engine && go build ./...
```

**Step 4: Commit**

```bash
git add luxview-engine/internal/repository/db.go
git commit -m "feat(analytics): add pageviews and pageview_aggregations tables"
```

---

### Task 3: Pageview Model

**Files:**
- Create: `luxview-engine/internal/model/pageview.go`

**Step 1: Create the model file**

```go
package model

import (
	"time"

	"github.com/google/uuid"
)

type Pageview struct {
	ID         int64      `json:"id"`
	AppID      *uuid.UUID `json:"app_id"`      // nil = platform (luxview.cloud)
	Timestamp  time.Time  `json:"timestamp"`
	Path       string     `json:"path"`
	Method     string     `json:"method"`
	StatusCode int        `json:"status_code"`
	IPHash     string     `json:"ip_hash"`
	Country    string     `json:"country,omitempty"`
	City       string     `json:"city,omitempty"`
	Region     string     `json:"region,omitempty"`
	Browser    string     `json:"browser,omitempty"`
	BrowserVer string     `json:"browser_ver,omitempty"`
	OS         string     `json:"os,omitempty"`
	DeviceType string     `json:"device_type,omitempty"` // desktop, mobile, tablet, bot
	Referer    string     `json:"referer,omitempty"`
	ResponseMs int        `json:"response_ms"`
}

type PageviewAggregation struct {
	ID            int64      `json:"id"`
	AppID         *uuid.UUID `json:"app_id"`
	Bucket        time.Time  `json:"bucket"`
	Granularity   string     `json:"granularity"` // "hour" or "day"
	Path          string     `json:"path,omitempty"`
	Views         int        `json:"views"`
	Visitors      int        `json:"visitors"`
	Bounces       int        `json:"bounces"`
	AvgDurationMs int        `json:"avg_duration_ms"`
	Country       string     `json:"country,omitempty"`
	Browser       string     `json:"browser,omitempty"`
	OS            string     `json:"os,omitempty"`
	DeviceType    string     `json:"device_type,omitempty"`
	RefererDomain string     `json:"referer_domain,omitempty"`
}
```

**Step 2: Verify build**

```bash
cd luxview-engine && go build ./...
```

**Step 3: Commit**

```bash
git add luxview-engine/internal/model/pageview.go
git commit -m "feat(analytics): add Pageview and PageviewAggregation models"
```

---

### Task 4: Pageview Repository

**Files:**
- Create: `luxview-engine/internal/repository/pageview_repo.go`

**Step 1: Create the repository**

```go
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
)

type PageviewRepo struct {
	db *DB
}

func NewPageviewRepo(db *DB) *PageviewRepo {
	return &PageviewRepo{db: db}
}

func (r *PageviewRepo) InsertBatch(ctx context.Context, pvs []model.Pageview) error {
	if len(pvs) == 0 {
		return nil
	}

	// Build multi-row INSERT (15 columns per row)
	const cols = 15
	q := `INSERT INTO pageviews (app_id, timestamp, path, method, status_code, ip_hash, country, city, region, browser, browser_ver, os, device_type, referer, response_ms) VALUES `
	args := make([]interface{}, 0, len(pvs)*cols)

	for i, pv := range pvs {
		if i > 0 {
			q += ", "
		}
		base := i * cols
		q += fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8,
			base+9, base+10, base+11, base+12, base+13, base+14, base+15)
		args = append(args, pv.AppID, pv.Timestamp, pv.Path, pv.Method, pv.StatusCode,
			pv.IPHash, pv.Country, pv.City, pv.Region, pv.Browser, pv.BrowserVer,
			pv.OS, pv.DeviceType, pv.Referer, pv.ResponseMs)
	}

	_, err := r.db.Pool.Exec(ctx, q, args...)
	return err
}

func (r *PageviewRepo) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM pageviews WHERE timestamp < $1`, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// CountSince returns pageview count and unique visitors since a given time for an app (or all if appID is nil).
func (r *PageviewRepo) CountSince(ctx context.Context, appID *uuid.UUID, since time.Time) (views int, visitors int, err error) {
	var q string
	var args []interface{}

	if appID != nil {
		q = `SELECT COUNT(*), COUNT(DISTINCT ip_hash) FROM pageviews WHERE app_id = $1 AND timestamp >= $2`
		args = []interface{}{appID, since}
	} else {
		q = `SELECT COUNT(*), COUNT(DISTINCT ip_hash) FROM pageviews WHERE timestamp >= $1`
		args = []interface{}{since}
	}

	err = r.db.Pool.QueryRow(ctx, q, args...).Scan(&views, &visitors)
	return
}
```

**Step 2: Verify build**

```bash
cd luxview-engine && go build ./...
```

**Step 3: Commit**

```bash
git add luxview-engine/internal/repository/pageview_repo.go
git commit -m "feat(analytics): add PageviewRepo with InsertBatch and CountSince"
```

---

### Task 5: GeoIP Service

**Files:**
- Create: `luxview-engine/internal/service/geoip.go`
- Modify: `luxview-engine/go.mod` (add maxminddb dependency)
- Modify: `luxview-engine/internal/config/config.go` (add GeoLite2 path)

**Step 1: Add dependency**

```bash
cd luxview-engine && go get github.com/oschwald/maxminddb-golang
```

**Step 2: Add config field**

In `config/config.go`, add to Config struct:

```go
GeoLite2Path string
```

In `Load()`, add:

```go
GeoLite2Path: envStr("GEOLITE2_PATH", "/usr/share/GeoIP/GeoLite2-City.mmdb"),
```

**Step 3: Create GeoIP service**

```go
package service

import (
	"net"
	"sync"

	"github.com/luxview/engine/pkg/logger"
	"github.com/oschwald/maxminddb-golang"
)

type GeoResult struct {
	Country string
	City    string
	Region  string
}

type GeoIP struct {
	reader *maxminddb.Reader
	mu     sync.RWMutex
}

func NewGeoIP(dbPath string) *GeoIP {
	log := logger.With("geoip")
	g := &GeoIP{}

	reader, err := maxminddb.Open(dbPath)
	if err != nil {
		log.Warn().Err(err).Str("path", dbPath).Msg("GeoLite2 database not found, geolocation disabled")
		return g
	}

	g.reader = reader
	log.Info().Str("path", dbPath).Msg("GeoLite2 database loaded")
	return g
}

func (g *GeoIP) Lookup(ipStr string) GeoResult {
	if g.reader == nil {
		return GeoResult{}
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return GeoResult{}
	}

	var record struct {
		Country struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
		City struct {
			Names map[string]string `maxminddb:"names"`
		} `maxminddb:"city"`
		Subdivisions []struct {
			Names map[string]string `maxminddb:"names"`
		} `maxminddb:"subdivisions"`
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	if err := g.reader.Lookup(ip, &record); err != nil {
		return GeoResult{}
	}

	result := GeoResult{Country: record.Country.ISOCode}
	if name, ok := record.City.Names["en"]; ok {
		result.City = name
	}
	if len(record.Subdivisions) > 0 {
		if name, ok := record.Subdivisions[0].Names["en"]; ok {
			result.Region = name
		}
	}
	return result
}

func (g *GeoIP) Close() {
	if g.reader != nil {
		g.reader.Close()
	}
}
```

**Step 4: Verify build**

```bash
cd luxview-engine && go build ./...
```

**Step 5: Commit**

```bash
git add luxview-engine/internal/service/geoip.go luxview-engine/internal/config/config.go luxview-engine/go.mod luxview-engine/go.sum
git commit -m "feat(analytics): add GeoIP service with MaxMind GeoLite2"
```

---

### Task 6: Log Parser Service

**Files:**
- Create: `luxview-engine/internal/service/log_parser.go`
- Modify: `luxview-engine/go.mod` (add useragent dependency)

**Step 1: Add dependency**

```bash
cd luxview-engine && go get github.com/mssola/useragent
```

**Step 2: Create log parser service**

```go
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
type TraefikLogEntry struct {
	ClientHost   string `json:"ClientHost"`
	RequestHost  string `json:"RequestHost"`
	RequestPath  string `json:"RequestPath"`
	RequestMethod string `json:"RequestMethod"`
	DownstreamStatus int `json:"DownstreamStatus"`
	Duration     int    `json:"Duration"` // nanoseconds
	StartUTC     string `json:"StartUTC"`
	RouterName   string `json:"RouterName"`
	RequestUserAgent  string `json:"request_User-Agent"`
	RequestReferer    string `json:"request_Referer"`
}

// LogParser converts Traefik log entries into Pageview models.
type LogParser struct {
	geoip       *GeoIP
	domain      string
	subdomainApps map[string]uuid.UUID // cache: subdomain → app_id
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

	// Filter: skip non-success/non-client-error responses
	if entry.DownstreamStatus < 200 || entry.DownstreamStatus >= 400 {
		// Allow 404 (page not found is still a visit)
		if entry.DownstreamStatus != 404 {
			return nil
		}
	}

	// Filter: skip static assets
	if lp.isStaticAsset(entry.RequestPath) {
		return nil
	}

	// Filter: skip internal paths
	if lp.isInternalPath(entry.RequestPath) {
		return nil
	}

	// Parse User-Agent
	ua := useragent.New(entry.RequestUserAgent)

	// Filter: skip bots
	if ua.Bot() {
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

	// IP hash for privacy
	ipHash := fmt.Sprintf("%x", sha256.Sum256([]byte(entry.ClientHost)))

	// Browser and OS
	browserName, browserVer := ua.Browser()
	osName := ua.OS()

	// Device type
	deviceType := "desktop"
	if ua.Mobile() {
		deviceType = "mobile"
	} else if ua.Tablet() {
		deviceType = "tablet"
	}

	// Referer domain
	referer := entry.RequestReferer
	if referer != "" {
		if u, err := url.Parse(referer); err == nil {
			// Skip self-referrals
			if strings.HasSuffix(u.Host, lp.domain) {
				referer = ""
			}
		}
	}

	// Duration: Traefik reports in nanoseconds
	responseMs := entry.Duration / 1_000_000

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

func (lp *LogParser) isStaticAsset(path string) bool {
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

func (lp *LogParser) isInternalPath(path string) bool {
	internals := []string{"/health", "/api/internal/", "/favicon.", "/robots.txt", "/sitemap.xml", "/manifest.json"}
	lower := strings.ToLower(path)
	for _, p := range internals {
		if strings.HasPrefix(lower, p) || lower == p {
			return true
		}
	}
	return false
}
```

**Step 3: Verify build**

```bash
cd luxview-engine && go build ./...
```

**Step 4: Commit**

```bash
git add luxview-engine/internal/service/log_parser.go luxview-engine/go.mod luxview-engine/go.sum
git commit -m "feat(analytics): add LogParser with UA parsing, GeoIP, and filtering"
```

---

### Task 7: Analytics Worker

**Files:**
- Create: `luxview-engine/internal/worker/analytics_worker.go`

**Step 1: Create the worker**

```go
package worker

import (
	"bufio"
	"context"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

const maxBatchSize = 1000

// AnalyticsWorker reads Traefik access logs and inserts pageviews.
type AnalyticsWorker struct {
	logPath      string
	parser       *service.LogParser
	pageviewRepo *repository.PageviewRepo
	appRepo      *repository.AppRepo
	interval     time.Duration
	offset       int64 // file read offset
}

func NewAnalyticsWorker(
	logPath string,
	parser *service.LogParser,
	pageviewRepo *repository.PageviewRepo,
	appRepo *repository.AppRepo,
	intervalSec int,
) *AnalyticsWorker {
	return &AnalyticsWorker{
		logPath:      logPath,
		parser:       parser,
		pageviewRepo: pageviewRepo,
		appRepo:      appRepo,
		interval:     time.Duration(intervalSec) * time.Second,
	}
}

func (aw *AnalyticsWorker) Start(ctx context.Context) {
	log := logger.With("analytics-worker")

	if aw.logPath == "" {
		log.Warn().Msg("no TRAEFIK_LOG_PATH configured, analytics worker disabled")
		return
	}

	log.Info().Str("log_path", aw.logPath).Dur("interval", aw.interval).Msg("starting analytics worker")

	// Seek to end of file on startup (don't process historical logs)
	if info, err := os.Stat(aw.logPath); err == nil {
		aw.offset = info.Size()
		log.Info().Int64("initial_offset", aw.offset).Msg("skipping existing log lines")
	}

	ticker := time.NewTicker(aw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("analytics worker stopped")
			return
		case <-ticker.C:
			aw.collect(ctx)
		}
	}
}

func (aw *AnalyticsWorker) collect(ctx context.Context) {
	log := logger.With("analytics-worker")

	// Refresh app cache
	apps, err := aw.appRepo.ListAll(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to list apps for analytics cache")
		return
	}

	appMap := make(map[string]uuid.UUID, len(apps))
	for _, app := range apps {
		appMap[app.Subdomain] = app.ID
	}
	aw.parser.UpdateAppCache(appMap)

	// Open log file
	f, err := os.Open(aw.logPath)
	if err != nil {
		log.Debug().Err(err).Msg("cannot open traefik log file")
		return
	}
	defer f.Close()

	// Handle file rotation (if file is smaller than offset, it was rotated)
	info, err := f.Stat()
	if err != nil {
		return
	}
	if info.Size() < aw.offset {
		aw.offset = 0
		log.Info().Msg("log file rotated, resetting offset")
	}

	// Seek to last position
	if _, err := f.Seek(aw.offset, io.SeekStart); err != nil {
		log.Error().Err(err).Msg("failed to seek log file")
		return
	}

	// Read new lines
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024) // 256KB max line

	var batch []model.Pageview
	linesRead := 0

	for scanner.Scan() {
		linesRead++
		pv := aw.parser.ParseLine(scanner.Bytes())
		if pv == nil {
			continue
		}
		batch = append(batch, *pv)

		if len(batch) >= maxBatchSize {
			if err := aw.pageviewRepo.InsertBatch(ctx, batch); err != nil {
				log.Error().Err(err).Int("count", len(batch)).Msg("failed to insert pageview batch")
			}
			batch = batch[:0]
		}
	}

	// Insert remaining
	if len(batch) > 0 {
		if err := aw.pageviewRepo.InsertBatch(ctx, batch); err != nil {
			log.Error().Err(err).Int("count", len(batch)).Msg("failed to insert pageview batch")
		}
	}

	// Update offset
	newOffset, _ := f.Seek(0, io.SeekCurrent)
	aw.offset = newOffset

	if linesRead > 0 {
		log.Debug().Int("lines", linesRead).Int("pageviews", len(batch)).Msg("analytics batch processed")
	}
}
```

**Step 2: Verify build**

```bash
cd luxview-engine && go build ./...
```

**Step 3: Commit**

```bash
git add luxview-engine/internal/worker/analytics_worker.go
git commit -m "feat(analytics): add AnalyticsWorker that reads Traefik logs and inserts pageviews"
```

---

### Task 8: Wire Everything in main.go

**Files:**
- Modify: `luxview-engine/cmd/engine/main.go`
- Modify: `luxview-engine/internal/config/config.go` (add TraefikLogPath + AnalyticsInterval)

**Step 1: Add config fields**

In `config.go` Config struct, add:

```go
TraefikLogPath    string
AnalyticsInterval int // seconds
```

In `Load()`, add:

```go
TraefikLogPath:    envStr("TRAEFIK_LOG_PATH", ""),
AnalyticsInterval: envInt("ANALYTICS_INTERVAL", 60),
```

**Step 2: Wire in main.go**

After existing workers section, add:

```go
// Analytics (GeoIP + log parser + worker)
geoipSvc := service.NewGeoIP(cfg.GeoLite2Path)
defer geoipSvc.Close()

logParser := service.NewLogParser(geoipSvc, cfg.Domain)
pageviewRepo := repository.NewPageviewRepo(db)

analyticsWorker := worker.NewAnalyticsWorker(
    cfg.TraefikLogPath,
    logParser,
    pageviewRepo,
    appRepo,
    cfg.AnalyticsInterval,
)
go analyticsWorker.Start(ctx)
```

**Step 3: Verify build**

```bash
cd luxview-engine && go build ./...
```

**Step 4: Commit**

```bash
git add luxview-engine/cmd/engine/main.go luxview-engine/internal/config/config.go
git commit -m "feat(analytics): wire GeoIP, LogParser, PageviewRepo, and AnalyticsWorker in main"
```

---

### Task 9: GeoLite2 Database Setup

**Files:**
- Modify: `luxview-engine/Dockerfile` (download GeoLite2 at build time)
- Create: `luxview-engine/scripts/download-geolite2.sh`

**Step 1: Create download script**

Since MaxMind requires a license key for direct download, use the open-source DB-IP alternative which is compatible with maxminddb format and requires no signup:

```bash
#!/bin/sh
# Download free GeoLite2-compatible City database from DB-IP
# License: CC BY 4.0 — https://db-ip.com/db/download/ip-to-city-lite
set -e

DEST_DIR="${1:-/usr/share/GeoIP}"
mkdir -p "$DEST_DIR"

MONTH=$(date +%Y-%m)
URL="https://download.db-ip.com/free/dbip-city-lite-${MONTH}.mmdb.gz"

echo "Downloading GeoIP database from $URL..."
wget -q -O /tmp/geoip.mmdb.gz "$URL"
gunzip -f /tmp/geoip.mmdb.gz
mv /tmp/geoip.mmdb "$DEST_DIR/GeoLite2-City.mmdb"
echo "GeoIP database installed at $DEST_DIR/GeoLite2-City.mmdb"
```

**Step 2: Update Dockerfile**

Add to the Dockerfile builder stage (or final stage):

```dockerfile
# Download GeoIP database
COPY scripts/download-geolite2.sh /tmp/download-geolite2.sh
RUN chmod +x /tmp/download-geolite2.sh && /tmp/download-geolite2.sh /usr/share/GeoIP
```

**Note:** Check the existing Dockerfile to determine the exact insertion point. This should be in the final stage so the DB is available at runtime.

**Step 3: Commit**

```bash
git add luxview-engine/scripts/download-geolite2.sh luxview-engine/Dockerfile
git commit -m "feat(analytics): add GeoLite2-compatible DB-IP download for geolocation"
```

---

### Task 10: AppRepo.ListAll Method

**Files:**
- Modify: `luxview-engine/internal/repository/app_repo.go` (add ListAll if missing)

**Step 1: Check if ListAll exists**

The AnalyticsWorker calls `appRepo.ListAll(ctx)`. Check `app_repo.go` — if only `ListAllRunning` exists, add:

```go
// ListAll returns all apps regardless of status.
func (r *AppRepo) ListAll(ctx context.Context) ([]model.App, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, user_id, name, subdomain, repo_url, repo_branch, stack, status,
		        container_id, internal_port, assigned_port, env_vars, resource_limits,
		        auto_deploy, created_at, updated_at, COALESCE(custom_dockerfile, '')
		 FROM apps ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list all apps: %w", err)
	}
	defer rows.Close()

	return scanApps(rows)
}
```

If `scanApps` doesn't exist as a shared helper, extract the row scanning logic from `ListAllRunning`.

**Step 2: Verify build and commit**

```bash
cd luxview-engine && go build ./... && git add luxview-engine/internal/repository/app_repo.go && git commit -m "feat(analytics): add AppRepo.ListAll for analytics subdomain cache"
```

---

### Verification

After all tasks, verify the full build:

```bash
cd luxview-engine && go build ./...
```

Then deploy to VPS:
1. Copy files to VPS
2. `docker compose build engine`
3. `docker compose up -d traefik engine`
4. Check engine logs: `docker logs luxview-engine --tail 20` — should see "starting analytics worker" and "GeoLite2 database loaded"
5. Wait 60s, check: `docker exec luxview-pg-platform psql -U luxview -d luxview_platform -c "SELECT COUNT(*) FROM pageviews;"`
