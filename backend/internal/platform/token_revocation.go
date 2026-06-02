package platform

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type TokenRevocation struct {
	rdb *redis.Client
}

func NewTokenRevocation(rdb *redis.Client) *TokenRevocation {
	return &TokenRevocation{rdb: rdb}
}

func (t *TokenRevocation) RevokeJTI(ctx context.Context, jti string, ttl time.Duration) error {
	key := "jwt:revoked:" + jti
	if err := t.rdb.Set(ctx, key, "1", ttl).Err(); err != nil {
		return fmt.Errorf("revoke jti: %w", err)
	}
	return nil
}

func (t *TokenRevocation) IsRevoked(ctx context.Context, jti string) (bool, error) {
	n, err := t.rdb.Exists(ctx, "jwt:revoked:"+jti).Result()
	if err != nil {
		return false, fmt.Errorf("check jti revoked: %w", err)
	}
	return n > 0, nil
}

type LoginRateLimiter struct {
	rdb      *redis.Client
	maxFails int
	window   time.Duration
}

func NewLoginRateLimiter(rdb *redis.Client, maxFails int, window time.Duration) *LoginRateLimiter {
	return &LoginRateLimiter{rdb: rdb, maxFails: maxFails, window: window}
}

func (l *LoginRateLimiter) key(email, ip string) string {
	return fmt.Sprintf("login:fail:%s:%s", email, ip)
}

func (l *LoginRateLimiter) IsLocked(ctx context.Context, email, ip string) (bool, error) {
	n, err := l.rdb.Get(ctx, l.key(email, ip)).Int()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return n >= l.maxFails, nil
}

func (l *LoginRateLimiter) RecordFailure(ctx context.Context, email, ip string) error {
	k := l.key(email, ip)
	pipe := l.rdb.Pipeline()
	incr := pipe.Incr(ctx, k)
	pipe.Expire(ctx, k, l.window)
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}
	_ = incr
	return nil
}

func (l *LoginRateLimiter) Clear(ctx context.Context, email, ip string) error {
	return l.rdb.Del(ctx, l.key(email, ip)).Err()
}
