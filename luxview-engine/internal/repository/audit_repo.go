package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type AuditLog struct {
	ID            int64      `json:"id"`
	ActorID       *uuid.UUID `json:"actorId"`
	ActorUsername  string     `json:"actorUsername"`
	Action        string     `json:"action"`
	ResourceType  string     `json:"resourceType"`
	ResourceID    string     `json:"resourceId,omitempty"`
	ResourceName  string     `json:"resourceName,omitempty"`
	OldValues     any        `json:"oldValues,omitempty"`
	NewValues     any        `json:"newValues,omitempty"`
	IPAddress     string     `json:"ipAddress,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

type AuditFilter struct {
	ActorID      *uuid.UUID
	Action       string
	ResourceType string
	ResourceID   string
	From         *time.Time
	To           *time.Time
	Search       string
}

type AuditLogRepo struct {
	db *DB
}

func NewAuditLogRepo(db *DB) *AuditLogRepo {
	return &AuditLogRepo{db: db}
}

func (r *AuditLogRepo) Create(ctx context.Context, log *AuditLog) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO audit_logs (actor_id, actor_username, action, resource_type, resource_id, resource_name, old_values, new_values, ip_address)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::INET)`,
		log.ActorID, log.ActorUsername, log.Action, log.ResourceType, log.ResourceID, log.ResourceName, log.OldValues, log.NewValues, nullIfEmpty(log.IPAddress),
	)
	if err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

func (r *AuditLogRepo) List(ctx context.Context, filter AuditFilter, limit, offset int) ([]AuditLog, error) {
	where, args := buildAuditWhere(filter)

	query := fmt.Sprintf(
		`SELECT id, actor_id, actor_username, action, resource_type, resource_id, resource_name, old_values, new_values, ip_address::TEXT, created_at
		 FROM audit_logs %s ORDER BY created_at DESC LIMIT %d OFFSET %d`,
		where, limit, offset,
	)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		var ipAddr *string
		if err := rows.Scan(&l.ID, &l.ActorID, &l.ActorUsername, &l.Action, &l.ResourceType, &l.ResourceID, &l.ResourceName, &l.OldValues, &l.NewValues, &ipAddr, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		if ipAddr != nil {
			l.IPAddress = *ipAddr
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func (r *AuditLogRepo) Count(ctx context.Context, filter AuditFilter) (int64, error) {
	where, args := buildAuditWhere(filter)
	query := fmt.Sprintf("SELECT COUNT(*) FROM audit_logs %s", where)

	var count int64
	if err := r.db.Pool.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count audit logs: %w", err)
	}
	return count, nil
}

func (r *AuditLogRepo) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM audit_logs WHERE created_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old audit logs: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *AuditLogRepo) StatsByAction(ctx context.Context, since time.Time) (map[string]int64, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT action, COUNT(*) FROM audit_logs WHERE created_at > $1 GROUP BY action`, since)
	if err != nil {
		return nil, fmt.Errorf("audit stats by action: %w", err)
	}
	defer rows.Close()
	return scanCountMap(rows)
}

func (r *AuditLogRepo) StatsByResource(ctx context.Context, since time.Time) (map[string]int64, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT resource_type, COUNT(*) FROM audit_logs WHERE created_at > $1 GROUP BY resource_type`, since)
	if err != nil {
		return nil, fmt.Errorf("audit stats by resource: %w", err)
	}
	defer rows.Close()
	return scanCountMap(rows)
}

func (r *AuditLogRepo) CountSince(ctx context.Context, since time.Time) (int64, error) {
	var count int64
	if err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE created_at > $1`, since).Scan(&count); err != nil {
		return 0, fmt.Errorf("count audit since: %w", err)
	}
	return count, nil
}

// --- helpers ---

func buildAuditWhere(f AuditFilter) (string, []any) {
	var conditions []string
	var args []any
	n := 1

	if f.ActorID != nil {
		conditions = append(conditions, fmt.Sprintf("actor_id = $%d", n))
		args = append(args, *f.ActorID)
		n++
	}
	if f.Action != "" {
		conditions = append(conditions, fmt.Sprintf("action = $%d", n))
		args = append(args, f.Action)
		n++
	}
	if f.ResourceType != "" {
		conditions = append(conditions, fmt.Sprintf("resource_type = $%d", n))
		args = append(args, f.ResourceType)
		n++
	}
	if f.ResourceID != "" {
		conditions = append(conditions, fmt.Sprintf("resource_id = $%d", n))
		args = append(args, f.ResourceID)
		n++
	}
	if f.From != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", n))
		args = append(args, *f.From)
		n++
	}
	if f.To != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", n))
		args = append(args, *f.To)
		n++
	}
	if f.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(actor_username ILIKE $%d OR resource_name ILIKE $%d)", n, n))
		args = append(args, "%"+f.Search+"%")
		n++
	}

	if len(conditions) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func scanCountMap(rows pgx.Rows) (map[string]int64, error) {
	result := make(map[string]int64)
	for rows.Next() {
		var key string
		var count int64
		if err := rows.Scan(&key, &count); err != nil {
			return nil, err
		}
		result[key] = count
	}
	return result, nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
