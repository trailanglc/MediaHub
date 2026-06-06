package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/anhtuanlc/mediahub/internal/platform/startup"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, dsn string, maxConns, minConns int) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}
	if maxConns <= 0 {
		maxConns = 20
	}
	if minConns <= 0 {
		minConns = 2
	}
	if minConns > maxConns {
		minConns = maxConns
	}
	cfg.MaxConns = int32(maxConns)
	cfg.MinConns = int32(minConns)
	cfg.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, startup.WrapPostgres(err, dsn)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, startup.WrapPostgres(err, dsn)
	}

	return pool, nil
}
