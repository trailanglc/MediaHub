package repository

import (
	"context"
	"encoding/json"
	"fmt"

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
