package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, action, target_type, target_id, ip, user_agent, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, actorID, action, targetType, targetID, ip, userAgent, meta)
	if err != nil {
		return fmt.Errorf("audit log: %w", err)
	}
	return nil
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
