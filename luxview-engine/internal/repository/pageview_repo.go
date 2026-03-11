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
