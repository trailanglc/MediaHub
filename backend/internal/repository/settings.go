package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const DefaultRootFolderPublicID = "00000000-0000-4000-8000-000000000001"

type SettingsRepository struct {
	pool *pgxpool.Pool
}

func NewSettingsRepository(pool *pgxpool.Pool) *SettingsRepository {
	return &SettingsRepository{pool: pool}
}

func (r *SettingsRepository) GetAll(ctx context.Context) (map[string]json.RawMessage, error) {
	rows, err := r.pool.Query(ctx, `SELECT key, value FROM system_settings`)
	if err != nil {
		if isUndefinedTable(err) {
			return map[string]json.RawMessage{}, nil
		}
		return nil, fmt.Errorf("settings get all: %w", err)
	}
	defer rows.Close()

	out := make(map[string]json.RawMessage)
	for rows.Next() {
		var key string
		var value json.RawMessage
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("settings scan: %w", err)
		}
		out[key] = value
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

type SettingUpsert struct {
	Key   string
	Value any
}

func (r *SettingsRepository) UpsertMany(ctx context.Context, items []SettingUpsert, updatedBy int64) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		if isUndefinedTable(err) {
			return ErrSettingsTableMissing
		}
		return fmt.Errorf("settings tx begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, item := range items {
		raw, err := json.Marshal(item.Value)
		if err != nil {
			return fmt.Errorf("settings marshal %s: %w", item.Key, err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO system_settings (key, value, updated_by, updated_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (key) DO UPDATE SET
				value = EXCLUDED.value,
				updated_by = EXCLUDED.updated_by,
				updated_at = EXCLUDED.updated_at
		`, item.Key, raw, updatedBy, time.Now().UTC())
		if err != nil {
			if isUndefinedTable(err) {
				return ErrSettingsTableMissing
			}
			return fmt.Errorf("settings upsert %s: %w", item.Key, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("settings tx commit: %w", err)
	}
	return nil
}

func (r *SettingsRepository) Get(ctx context.Context, key string) (json.RawMessage, error) {
	var value json.RawMessage
	err := r.pool.QueryRow(ctx, `SELECT value FROM system_settings WHERE key = $1`, key).Scan(&value)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrSettingNotFound
		}
		return nil, fmt.Errorf("settings get: %w", err)
	}
	return value, nil
}

var (
	ErrSettingNotFound      = errors.New("setting not found")
	ErrSettingsTableMissing = errors.New("system_settings table missing; run database migrations (make migrate-up)")
)

func isUndefinedTable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42P01"
}
