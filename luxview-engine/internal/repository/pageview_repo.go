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

// CountSince returns pageview count and unique visitors since a given time.
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

// ---- Analytics query methods ----

type OverviewRow struct {
	Bucket   time.Time `json:"bucket"`
	Views    int       `json:"views"`
	Visitors int       `json:"visitors"`
}

type OverviewResult struct {
	Visitors    int           `json:"visitors"`
	Pageviews   int           `json:"pageviews"`
	BounceRate  float64       `json:"bounce_rate"`
	AvgDuration int           `json:"avg_duration_ms"`
	TimeSeries  []OverviewRow `json:"time_series"`
	PrevVisitors  int         `json:"prev_visitors"`
	PrevPageviews int         `json:"prev_pageviews"`
}

func (r *PageviewRepo) Overview(ctx context.Context, appID *uuid.UUID, start, end time.Time, granularity string) (*OverviewResult, error) {
	duration := end.Sub(start)
	prevStart := start.Add(-duration)
	prevEnd := start

	// Decide source table
	useRaw := duration <= 7*24*time.Hour

	result := &OverviewResult{}

	if useRaw {
		// KPIs from raw
		q := `SELECT COUNT(*), COUNT(DISTINCT ip_hash), COALESCE(AVG(response_ms)::int, 0) FROM pageviews WHERE timestamp >= $1 AND timestamp < $2`
		args := []interface{}{start, end}
		if appID != nil {
			q += ` AND app_id = $3`
			args = append(args, *appID)
		} else {
			q += ` AND app_id IS NULL`
		}
		err := r.db.Pool.QueryRow(ctx, q, args...).Scan(&result.Pageviews, &result.Visitors, &result.AvgDuration)
		if err != nil {
			return nil, fmt.Errorf("overview kpis: %w", err)
		}

		// Previous period
		qPrev := `SELECT COUNT(*), COUNT(DISTINCT ip_hash) FROM pageviews WHERE timestamp >= $1 AND timestamp < $2`
		argsPrev := []interface{}{prevStart, prevEnd}
		if appID != nil {
			qPrev += ` AND app_id = $3`
			argsPrev = append(argsPrev, *appID)
		} else {
			qPrev += ` AND app_id IS NULL`
		}
		_ = r.db.Pool.QueryRow(ctx, qPrev, argsPrev...).Scan(&result.PrevPageviews, &result.PrevVisitors)

		// Bounce rate (ip_hash with single pageview in window)
		qBounce := `SELECT COUNT(*) FROM (
			SELECT ip_hash FROM pageviews WHERE timestamp >= $1 AND timestamp < $2`
		argsBounce := []interface{}{start, end}
		if appID != nil {
			qBounce += ` AND app_id = $3`
			argsBounce = append(argsBounce, *appID)
		} else {
			qBounce += ` AND app_id IS NULL`
		}
		qBounce += ` GROUP BY ip_hash HAVING COUNT(*) = 1) AS bounced`
		var bounced int
		_ = r.db.Pool.QueryRow(ctx, qBounce, argsBounce...).Scan(&bounced)
		if result.Visitors > 0 {
			result.BounceRate = float64(bounced) / float64(result.Visitors) * 100
		}

		// Time series
		trunc := "hour"
		if granularity == "day" {
			trunc = "day"
		}
		qTS := fmt.Sprintf(`SELECT date_trunc('%s', timestamp) AS bucket, COUNT(*), COUNT(DISTINCT ip_hash)
			FROM pageviews WHERE timestamp >= $1 AND timestamp < $2`, trunc)
		argsTS := []interface{}{start, end}
		if appID != nil {
			qTS += ` AND app_id = $3`
			argsTS = append(argsTS, *appID)
		} else {
			qTS += ` AND app_id IS NULL`
		}
		qTS += ` GROUP BY bucket ORDER BY bucket`

		rows, err := r.db.Pool.Query(ctx, qTS, argsTS...)
		if err != nil {
			return nil, fmt.Errorf("overview timeseries: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var row OverviewRow
			if err := rows.Scan(&row.Bucket, &row.Views, &row.Visitors); err != nil {
				return nil, err
			}
			result.TimeSeries = append(result.TimeSeries, row)
		}
	} else {
		// From aggregations
		q := `SELECT COALESCE(SUM(views), 0), COALESCE(SUM(visitors), 0), COALESCE(AVG(avg_duration_ms)::int, 0)
			FROM pageview_aggregations WHERE bucket >= $1 AND bucket < $2`
		args := []interface{}{start, end}
		if appID != nil {
			q += ` AND app_id = $3`
			args = append(args, *appID)
		} else {
			q += ` AND app_id IS NULL`
		}
		err := r.db.Pool.QueryRow(ctx, q, args...).Scan(&result.Pageviews, &result.Visitors, &result.AvgDuration)
		if err != nil {
			return nil, fmt.Errorf("overview agg kpis: %w", err)
		}

		// Previous period from aggregations
		qPrev := `SELECT COALESCE(SUM(views), 0), COALESCE(SUM(visitors), 0)
			FROM pageview_aggregations WHERE bucket >= $1 AND bucket < $2`
		argsPrev := []interface{}{prevStart, prevEnd}
		if appID != nil {
			qPrev += ` AND app_id = $3`
			argsPrev = append(argsPrev, *appID)
		} else {
			qPrev += ` AND app_id IS NULL`
		}
		_ = r.db.Pool.QueryRow(ctx, qPrev, argsPrev...).Scan(&result.PrevPageviews, &result.PrevVisitors)

		// Bounce rate from aggregations
		qBounce := `SELECT COALESCE(SUM(bounces), 0) FROM pageview_aggregations WHERE bucket >= $1 AND bucket < $2`
		argsBounce := []interface{}{start, end}
		if appID != nil {
			qBounce += ` AND app_id = $3`
			argsBounce = append(argsBounce, *appID)
		} else {
			qBounce += ` AND app_id IS NULL`
		}
		var bounced int
		_ = r.db.Pool.QueryRow(ctx, qBounce, argsBounce...).Scan(&bounced)
		if result.Visitors > 0 {
			result.BounceRate = float64(bounced) / float64(result.Visitors) * 100
		}

		// Time series from aggregations
		gran := "hour"
		if granularity == "day" {
			gran = "day"
		}
		qTS := `SELECT bucket, SUM(views), SUM(visitors) FROM pageview_aggregations
			WHERE bucket >= $1 AND bucket < $2 AND granularity = $3`
		argsTS := []interface{}{start, end, gran}
		if appID != nil {
			qTS += ` AND app_id = $4`
			argsTS = append(argsTS, *appID)
		} else {
			qTS += ` AND app_id IS NULL`
		}
		qTS += ` GROUP BY bucket ORDER BY bucket`

		rows, err := r.db.Pool.Query(ctx, qTS, argsTS...)
		if err != nil {
			return nil, fmt.Errorf("overview agg timeseries: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var row OverviewRow
			if err := rows.Scan(&row.Bucket, &row.Views, &row.Visitors); err != nil {
				return nil, err
			}
			result.TimeSeries = append(result.TimeSeries, row)
		}
	}

	return result, nil
}

type RankedItem struct {
	Name     string `json:"name"`
	Views    int    `json:"views"`
	Visitors int    `json:"visitors"`
}

func (r *PageviewRepo) TopPages(ctx context.Context, appID *uuid.UUID, start, end time.Time, limit int) ([]RankedItem, error) {
	q := `SELECT path, COUNT(*) AS views, COUNT(DISTINCT ip_hash) AS visitors
		FROM pageviews WHERE timestamp >= $1 AND timestamp < $2`
	args := []interface{}{start, end}
	if appID != nil {
		q += ` AND app_id = $3`
		args = append(args, *appID)
	} else {
		q += ` AND app_id IS NULL`
	}
	q += fmt.Sprintf(` GROUP BY path ORDER BY views DESC LIMIT %d`, limit)

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []RankedItem
	for rows.Next() {
		var item RankedItem
		if err := rows.Scan(&item.Name, &item.Views, &item.Visitors); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *PageviewRepo) TopGeo(ctx context.Context, appID *uuid.UUID, start, end time.Time, limit int) ([]RankedItem, error) {
	q := `SELECT COALESCE(NULLIF(country, ''), 'unknown'), COUNT(*) AS views, COUNT(DISTINCT ip_hash) AS visitors
		FROM pageviews WHERE timestamp >= $1 AND timestamp < $2`
	args := []interface{}{start, end}
	if appID != nil {
		q += ` AND app_id = $3`
		args = append(args, *appID)
	} else {
		q += ` AND app_id IS NULL`
	}
	q += fmt.Sprintf(` GROUP BY country ORDER BY visitors DESC LIMIT %d`, limit)

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []RankedItem
	for rows.Next() {
		var item RankedItem
		if err := rows.Scan(&item.Name, &item.Views, &item.Visitors); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *PageviewRepo) Breakdown(ctx context.Context, appID *uuid.UUID, start, end time.Time, column string) ([]RankedItem, error) {
	// Validate column to prevent SQL injection
	validColumns := map[string]bool{"browser": true, "os": true, "device_type": true}
	if !validColumns[column] {
		return nil, fmt.Errorf("invalid breakdown column: %s", column)
	}

	q := fmt.Sprintf(`SELECT COALESCE(NULLIF(%s, ''), 'unknown'), COUNT(*) AS views, COUNT(DISTINCT ip_hash) AS visitors
		FROM pageviews WHERE timestamp >= $1 AND timestamp < $2`, column)
	args := []interface{}{start, end}
	if appID != nil {
		q += ` AND app_id = $3`
		args = append(args, *appID)
	} else {
		q += ` AND app_id IS NULL`
	}
	q += fmt.Sprintf(` GROUP BY %s ORDER BY visitors DESC`, column)

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []RankedItem
	for rows.Next() {
		var item RankedItem
		if err := rows.Scan(&item.Name, &item.Views, &item.Visitors); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *PageviewRepo) TopReferers(ctx context.Context, appID *uuid.UUID, start, end time.Time, limit int) ([]RankedItem, error) {
	q := `SELECT COALESCE(NULLIF(referer, ''), 'direct'), COUNT(*) AS views, COUNT(DISTINCT ip_hash) AS visitors
		FROM pageviews WHERE timestamp >= $1 AND timestamp < $2`
	args := []interface{}{start, end}
	if appID != nil {
		q += ` AND app_id = $3`
		args = append(args, *appID)
	} else {
		q += ` AND app_id IS NULL`
	}
	q += fmt.Sprintf(` GROUP BY referer ORDER BY visitors DESC LIMIT %d`, limit)

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []RankedItem
	for rows.Next() {
		var item RankedItem
		if err := rows.Scan(&item.Name, &item.Views, &item.Visitors); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// LiveVisitors returns the count of unique visitors in the last N minutes.
func (r *PageviewRepo) LiveVisitors(ctx context.Context, appID *uuid.UUID, minutes int) (int, error) {
	since := time.Now().Add(-time.Duration(minutes) * time.Minute)
	q := `SELECT COUNT(DISTINCT ip_hash) FROM pageviews WHERE timestamp >= $1`
	args := []interface{}{since}
	if appID != nil {
		q += ` AND app_id = $2`
		args = append(args, *appID)
	} else {
		q += ` AND app_id IS NULL`
	}

	var count int
	err := r.db.Pool.QueryRow(ctx, q, args...).Scan(&count)
	return count, err
}

// ---- Aggregation methods ----

// AggregateHourly aggregates raw pageviews into hourly buckets for a time range.
func (r *PageviewRepo) AggregateHourly(ctx context.Context, before time.Time) (int64, error) {
	q := `INSERT INTO pageview_aggregations (app_id, bucket, granularity, path, views, visitors, bounces, avg_duration_ms, country, browser, os, device_type, referer_domain)
		SELECT app_id, date_trunc('hour', timestamp) AS bucket, 'hour',
			path, COUNT(*) AS views, COUNT(DISTINCT ip_hash) AS visitors,
			0, COALESCE(AVG(response_ms), 0)::int,
			country, browser, os, device_type,
			CASE WHEN referer = '' THEN '' ELSE split_part(referer, '/', 3) END
		FROM pageviews
		WHERE timestamp < $1
		GROUP BY app_id, bucket, path, country, browser, os, device_type,
			CASE WHEN referer = '' THEN '' ELSE split_part(referer, '/', 3) END
		ON CONFLICT (app_id, bucket, granularity, COALESCE(path, ''), COALESCE(country, ''), COALESCE(browser, ''), COALESCE(os, ''), COALESCE(device_type, ''), COALESCE(referer_domain, ''))
		DO UPDATE SET views = EXCLUDED.views, visitors = EXCLUDED.visitors, avg_duration_ms = EXCLUDED.avg_duration_ms`

	tag, err := r.db.Pool.Exec(ctx, q, before)
	if err != nil {
		return 0, fmt.Errorf("aggregate hourly: %w", err)
	}
	return tag.RowsAffected(), nil
}

// CompactToDaily compacts hourly aggregations into daily for data older than the given threshold.
func (r *PageviewRepo) CompactToDaily(ctx context.Context, before time.Time) (int64, error) {
	q := `INSERT INTO pageview_aggregations (app_id, bucket, granularity, path, views, visitors, bounces, avg_duration_ms, country, browser, os, device_type, referer_domain)
		SELECT app_id, date_trunc('day', bucket) AS day_bucket, 'day',
			path, SUM(views), SUM(visitors), SUM(bounces),
			CASE WHEN SUM(views) > 0 THEN (SUM(avg_duration_ms::bigint * views) / SUM(views))::int ELSE 0 END,
			country, browser, os, device_type, referer_domain
		FROM pageview_aggregations
		WHERE granularity = 'hour' AND bucket < $1
		GROUP BY app_id, day_bucket, path, country, browser, os, device_type, referer_domain
		ON CONFLICT (app_id, bucket, granularity, COALESCE(path, ''), COALESCE(country, ''), COALESCE(browser, ''), COALESCE(os, ''), COALESCE(device_type, ''), COALESCE(referer_domain, ''))
		DO UPDATE SET views = EXCLUDED.views, visitors = EXCLUDED.visitors, bounces = EXCLUDED.bounces, avg_duration_ms = EXCLUDED.avg_duration_ms`

	tag, err := r.db.Pool.Exec(ctx, q, before)
	if err != nil {
		return 0, fmt.Errorf("compact to daily: %w", err)
	}
	return tag.RowsAffected(), nil
}

// DeleteHourlyOlderThan removes hourly aggregations older than the given time.
func (r *PageviewRepo) DeleteHourlyOlderThan(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM pageview_aggregations WHERE granularity = 'hour' AND bucket < $1`, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// DeleteDailyOlderThan removes daily aggregations older than the given time.
func (r *PageviewRepo) DeleteDailyOlderThan(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM pageview_aggregations WHERE granularity = 'day' AND bucket < $1`, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
