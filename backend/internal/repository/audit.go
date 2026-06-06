package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anhtuanlc/mediahub/internal/platform/clientip"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{pool: pool}
}

func (r *AuditRepository) Log(ctx context.Context, actorID *int64, action, targetType string, targetID *int64, ip, userAgent string, metadata map[string]string) error {
	var meta []byte
	if metadata != nil {
		meta, _ = json.Marshal(metadata)
	} else {
		meta = []byte("{}")
	}
	ip = clientip.Normalize(ip)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, action, target_type, target_id, ip, user_agent, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, actorID, action, targetType, targetID, ip, userAgent, meta)
	if err != nil {
		return fmt.Errorf("audit log: %w", err)
	}
	return nil
}

// AuditEntry is a row from audit_logs with optional actor details.
type AuditEntry struct {
	ID             int64
	ActorID        *int64
	Action         string
	TargetType     *string
	TargetID       *int64
	IP             *string
	UserAgent      *string
	Metadata       map[string]string
	CreatedAt      time.Time
	ActorEmail     *string
	ActorPublicID  *uuid.UUID
}

// ListAuditFilters narrows audit log queries.
type ListAuditFilters struct {
	Actions         []string
	ActionPrefix    string
	ActorID         int64
	ActorPublicID   *uuid.UUID
	From            *time.Time
	To              *time.Time
}

// List returns audit entries in reverse chronological order (cursor = last seen id, 0 for first page).
func (r *AuditRepository) List(ctx context.Context, cursor int64, limit int, filters ListAuditFilters) ([]AuditEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	actions := filters.Actions
	if actions == nil {
		actions = []string{}
	}

	rows, err := r.pool.Query(ctx, `
		SELECT a.id, a.actor_id, a.action, a.target_type, a.target_id, a.ip, a.user_agent, a.metadata, a.created_at,
		       u.email, u.public_id
		FROM audit_logs a
		LEFT JOIN users u ON u.id = a.actor_id
		WHERE ($1 = 0 OR a.id < $1)
		  AND (cardinality($2::text[]) = 0 OR a.action = ANY($2))
		  AND ($3 = '' OR a.action LIKE $3 || '%')
		  AND ($4 = 0 OR a.actor_id = $4)
		  AND ($5::uuid IS NULL OR u.public_id = $5)
		  AND ($6::timestamptz IS NULL OR a.created_at >= $6)
		  AND ($7::timestamptz IS NULL OR a.created_at <= $7)
		ORDER BY a.id DESC
		LIMIT $8
	`, cursor, actions, filters.ActionPrefix, filters.ActorID, filters.ActorPublicID, filters.From, filters.To, limit)
	if err != nil {
		return nil, fmt.Errorf("audit list: %w", err)
	}
	defer rows.Close()

	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var targetType, ip, ua *string
		var meta []byte
		var actorEmail *string
		var actorPID *uuid.UUID
		if err := rows.Scan(
			&e.ID, &e.ActorID, &e.Action, &targetType, &e.TargetID, &ip, &ua, &meta, &e.CreatedAt,
			&actorEmail, &actorPID,
		); err != nil {
			return nil, fmt.Errorf("audit scan: %w", err)
		}
		e.TargetType = targetType
		e.IP = ip
		e.UserAgent = ua
		e.ActorEmail = actorEmail
		e.ActorPublicID = actorPID
		e.Metadata = decodeAuditMetadata(meta)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit list rows: %w", err)
	}
	return out, nil
}

// ListDistinctActions returns distinct audit action values sorted alphabetically.
func (r *AuditRepository) ListDistinctActions(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT action FROM audit_logs ORDER BY action ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("audit distinct actions: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			return nil, fmt.Errorf("audit distinct actions scan: %w", err)
		}
		out = append(out, action)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit distinct actions rows: %w", err)
	}
	return out, nil
}

func decodeAuditMetadata(raw []byte) map[string]string {
	if len(raw) == 0 {
		return map[string]string{}
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return map[string]string{}
	}
	return m
}

const defaultAuditDeleteBatch = 1000

// DeleteOlderThan removes audit rows with created_at before cutoff, in batches.
func (r *AuditRepository) DeleteOlderThan(ctx context.Context, before time.Time, batchSize int) (int64, error) {
	if batchSize <= 0 {
		batchSize = defaultAuditDeleteBatch
	}
	var total int64
	for {
		tag, err := r.pool.Exec(ctx, `
			DELETE FROM audit_logs
			WHERE id IN (
				SELECT id FROM audit_logs
				WHERE created_at < $1
				ORDER BY id
				LIMIT $2
			)
		`, before, batchSize)
		if err != nil {
			return total, fmt.Errorf("audit delete older than: %w", err)
		}
		n := tag.RowsAffected()
		total += n
		if n < int64(batchSize) {
			break
		}
	}
	return total, nil
}
