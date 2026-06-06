package runtime

import (
	"context"

	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/anhtuanlc/mediahub/internal/platform/postgres"
	platredis "github.com/anhtuanlc/mediahub/internal/platform/redis"
	"github.com/anhtuanlc/mediahub/internal/platform/startup"
	"github.com/anhtuanlc/mediahub/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Shared holds connections verified at bootstrap and reused by all backend components.
type Shared struct {
	Cfg    *config.Config
	Logger *zap.Logger
	Pool   *pgxpool.Pool
	Redis  *redis.Client
	Store  *storage.S3Storage
}

// Bootstrap loads config dependencies, verifies Postgres/Redis/MinIO and schema, then returns shared resources.
func Bootstrap(ctx context.Context, cfg *config.Config, logger *zap.Logger) (*Shared, error) {
	pool, err := postgres.NewPool(ctx, cfg.DBDSN, cfg.DBMaxConns, cfg.DBMinConns)
	if err != nil {
		return nil, err
	}

	if err := checkSchema(ctx, pool, logger); err != nil {
		pool.Close()
		return nil, err
	}

	redisClient := platredis.NewClientFromConfig(cfg)
	if err := platredis.Ping(ctx, redisClient); err != nil {
		pool.Close()
		return nil, startup.WrapRedis(err, cfg.RedisAddr)
	}

	store, err := storage.NewS3Storage(ctx, cfg.Storage, cfg.HealthMetricsCacheTTL)
	if err != nil {
		pool.Close()
		_ = redisClient.Close()
		return nil, err
	}
	if err := store.Ping(ctx); err != nil {
		pool.Close()
		_ = redisClient.Close()
		return nil, startup.WrapMinIO(err, cfg.Storage.Endpoint)
	}
	if cfg.AppEnv == "development" {
		if err := store.EnsureBucket(ctx); err != nil {
			pool.Close()
			_ = redisClient.Close()
			return nil, startup.WrapMinIO(err, cfg.Storage.Endpoint)
		}
	}

	logger.Info("bootstrap ok",
		zap.String("postgres", "connected"),
		zap.String("redis", cfg.RedisAddr),
		zap.String("storage", cfg.Storage.Endpoint),
	)

	return &Shared{
		Cfg:    cfg,
		Logger: logger,
		Pool:   pool,
		Redis:  redisClient,
		Store:  store,
	}, nil
}

func checkSchema(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) error {
	var settingsTable bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'system_settings'
		)
	`).Scan(&settingsTable); err != nil {
		return err
	}
	if !settingsTable {
		return &startup.DependencyError{
			Service: "PostgreSQL (schema)",
			Detail:  "bảng system_settings chưa tồn tại",
			Fix:     "Chạy: make migrate-up\nHoặc: make dev-full (infra + migrate + gateway)",
		}
	}

	var uploadSessionsTable bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'upload_sessions'
		)
	`).Scan(&uploadSessionsTable); err != nil {
		logger.Warn("upload_sessions table check failed", zap.Error(err))
	} else if !uploadSessionsTable {
		logger.Warn("upload_sessions table missing — run: make migrate-up")
	}

	return nil
}

// Close releases bootstrap connections.
func (s *Shared) Close() {
	if s == nil {
		return
	}
	if s.Pool != nil {
		s.Pool.Close()
	}
	if s.Redis != nil {
		_ = s.Redis.Close()
	}
}
