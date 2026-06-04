package redis

import (
	"context"
	"time"

	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/redis/go-redis/v9"
)

const (
	defaultPoolSize        = 32
	defaultMinIdleConns    = 8
	defaultRedisTimeout    = 3 * time.Second
)

func NewClient(addr string) *redis.Client {
	return NewClientFromConfig(&config.Config{RedisAddr: addr})
}

func NewClientFromConfig(cfg *config.Config) *redis.Client {
	opts := &redis.Options{
		Addr:         cfg.RedisAddr,
		PoolSize:     cfg.RedisPoolSize,
		MinIdleConns: cfg.RedisMinIdleConns,
		ReadTimeout:  cfg.RedisReadTimeout,
		WriteTimeout: cfg.RedisWriteTimeout,
	}
	if opts.PoolSize <= 0 {
		opts.PoolSize = defaultPoolSize
	}
	if opts.MinIdleConns <= 0 {
		opts.MinIdleConns = defaultMinIdleConns
	}
	if opts.ReadTimeout <= 0 {
		opts.ReadTimeout = defaultRedisTimeout
	}
	if opts.WriteTimeout <= 0 {
		opts.WriteTimeout = defaultRedisTimeout
	}
	return redis.NewClient(opts)
}

func Ping(ctx context.Context, client *redis.Client) error {
	return client.Ping(ctx).Err()
}
