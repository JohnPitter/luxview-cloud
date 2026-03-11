# Platform Analytics — Design Document

**Date:** 2026-03-11
**Status:** Approved

## Overview

Global usage metrics for the LuxView Cloud platform, tracking visitors for both the platform dashboard (`luxview.cloud`) and user-deployed apps (`*.luxview.cloud`). Data is collected passively from Traefik access logs — zero impact on user apps.

## Architecture

```
Traefik (access.json) → Analytics Worker (60s batch) → pageviews table
                              ↓                              ↓
                         GeoLite2 lookup              Aggregation Worker (daily)
                         UA parsing                        ↓
                                                   pageview_aggregations table
                                                          ↓
                                                   API endpoints → Dashboard UI
```

### Data Collection

- **Source:** Traefik access logs written to shared volume (`/var/log/traefik/access.json`)
- **GeoIP:** MaxMind GeoLite2 offline database (microsecond lookups, no external dependency)
- **UA Parsing:** `mssola/useragent` Go library
- **Privacy:** IP stored as SHA256 hash (LGPD compliant), sufficient for unique visitor counting

### Analytics Worker (every 60s)

1. Read new lines from access.json (track file offset with seek)
2. Parse each JSON line: host, path, method, status, IP, user-agent, referer, duration
3. Determine app_id by host (subdomain → app lookup, luxview.cloud → NULL for platform)
4. Enrich: IP → hash + GeoLite2, User-Agent → browser/OS/device
5. Bulk INSERT into DB (batch up to 1000 rows)

**Filters (to avoid polluting data):**
- Ignore known bots (Googlebot, curl, wget) via UA
- Ignore asset paths (`/assets/`, `.js`, `.css`, `.png`, `.svg`, `.ico`)
- Ignore health checks (`/health`, `/api/internal/`)
- Ignore redirects (301/302)
- Only count GET requests (pageviews, not API calls)

### Aggregation Worker (daily at 00:00 UTC)

1. Aggregate pageviews older than 7 days into hourly buckets
2. Compact hourly aggregations older than 30 days into daily buckets
3. Purge expired data (raw > 7 days, aggregations > 90 days)

**Bounce detection:** ip_hash with only 1 pageview within a 30-minute window counts as bounce.

**Storage estimate (~1000 visits/day per app):**
- Raw (7 days): ~2MB
- Hourly (30 days): ~500KB
- Daily (90 days): ~50KB
- Total per app: ~3MB

## Database Schema

```sql
CREATE TABLE pageviews (
    id          BIGSERIAL PRIMARY KEY,
    app_id      UUID REFERENCES apps(id) ON DELETE CASCADE,
    timestamp   TIMESTAMPTZ NOT NULL,
    path        VARCHAR(2048) NOT NULL,
    method      VARCHAR(10) NOT NULL,
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
    response_ms SMALLINT
);

CREATE TABLE pageview_aggregations (
    id          BIGSERIAL PRIMARY KEY,
    app_id      UUID REFERENCES apps(id) ON DELETE CASCADE,
    bucket      TIMESTAMPTZ NOT NULL,
    granularity VARCHAR(4) NOT NULL,
    path        VARCHAR(2048),
    views       INTEGER NOT NULL DEFAULT 0,
    visitors    INTEGER NOT NULL DEFAULT 0,
    bounces     INTEGER NOT NULL DEFAULT 0,
    avg_duration_ms INTEGER,
    country     VARCHAR(2),
    browser     VARCHAR(64),
    os          VARCHAR(64),
    device_type VARCHAR(16),
    referer_domain VARCHAR(256),
    UNIQUE(app_id, bucket, granularity, path, country, browser, os, device_type, referer_domain)
);

CREATE INDEX idx_pv_app_ts ON pageviews(app_id, timestamp DESC);
CREATE INDEX idx_pv_app_ip ON pageviews(app_id, ip_hash, timestamp);
CREATE INDEX idx_pva_app_bucket ON pageview_aggregations(app_id, bucket DESC, granularity);
```

## API Endpoints

All under `/api/analytics`, protected by auth.

| Endpoint | Description |
|---|---|
| `GET /overview?app_id=&start=&end=&granularity=hour\|day` | KPIs + time series (visitors, pageviews, bounceRate, avgDuration) |
| `GET /pages?app_id=&start=&end=&limit=20` | Top pages by views |
| `GET /geo?app_id=&start=&end=&limit=20` | Top countries/cities |
| `GET /browsers?app_id=&start=&end=` | Browser breakdown |
| `GET /os?app_id=&start=&end=` | OS breakdown |
| `GET /devices?app_id=&start=&end=` | Device type breakdown |
| `GET /referers?app_id=&start=&end=&limit=20` | Top referrer domains |
| `GET /live` | Active visitors (last 5 min) |

**Permissions:**
- `app_id` omitted → platform data (admin only)
- `app_id` present → app data (app owner or admin)
- `app_id=all` → aggregated across user's apps

**Performance:**
- Queries ≤ 7 days → `pageviews` table (raw data)
- Queries > 7 days → `pageview_aggregations` table
- All endpoints: `Cache-Control: max-age=60`

## Frontend — `/dashboard/analytics`

**Header:**
- App selector dropdown (user's apps + "Platform" for admins)
- Date range picker (today, 7d, 30d, 90d, custom)
- Live counter (pulsing green dot + "X visitors now")

**Row 1 — KPI Cards (4x GlassCard):**
- Unique visitors (with % change vs previous period)
- Total pageviews
- Bounce rate
- Average duration

**Row 2 — Main chart:**
- Line chart: visitors + pageviews over time (Recharts)
- Hour/day granularity toggle

**Row 3 — Two-column grid:**
- Left: Top countries (flag emoji + progress bar)
- Right: Top pages (table: path, views, visitors)

**Row 4 — Three-column grid (donut charts):**
- Browsers (Chrome, Firefox, Safari, Edge, other)
- Operating systems (Windows, macOS, Linux, Android, iOS)
- Devices (Desktop, Mobile, Tablet)

**Row 5 — Referrers:**
- Table: domain, visitors, pageviews
- Favicon via `google.com/s2/favicons`

## Configuration Changes

**Traefik (`traefik.yml`):**
- Add `filePath: /var/log/traefik/access.json` to accessLog
- Add User-Agent and Referer to logged fields

**Docker Compose:**
- Add shared volume `traefik-logs` between traefik and engine containers
- Mount GeoLite2 database file into engine container

## Go Dependencies

- `github.com/oschwald/maxminddb-golang` — GeoLite2 reader
- `github.com/mssola/useragent` — User-Agent parsing

## Implementation Phases

### Phase A: Infrastructure (backend)
- Traefik log config + shared volume
- DB tables (pageviews, pageview_aggregations)
- GeoLite2 setup (download + mount)
- Analytics worker (log parser + enrichment + bulk insert)

### Phase B: API + Aggregation
- Aggregation worker (hourly/daily compaction + cleanup)
- API endpoints (overview, pages, geo, browsers, os, devices, referers, live)
- Permission checks (admin vs app owner)

### Phase C: Frontend
- Analytics page layout + routing
- KPI cards + main chart
- Geo, browser, OS, device visualizations
- Referrers table
- Live counter
- App selector + date range picker

### Phase D: Polish
- i18n (pt-BR, en, es)
- Loading skeletons
- Empty states
- Mobile responsive
