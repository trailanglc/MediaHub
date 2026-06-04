package rediscache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anhtuanlc/mediahub/internal/platform/resource"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// Store provides JSON cache helpers and prefix invalidation on Redis.
type Store struct {
	rdb       *redis.Client
	sf        singleflight.Group
	Resources *resource.Reader
}

func NewStore(rdb *redis.Client) *Store {
	if rdb == nil {
		return nil
	}
	return &Store{rdb: rdb}
}

// WithResources attaches a resource reader for adaptive TTL / skip-set under pressure.
func (s *Store) WithResources(r *resource.Reader) *Store {
	if s == nil {
		return nil
	}
	s.Resources = r
	return s
}

func (s *Store) effectiveTTL(ctx context.Context, base time.Duration) time.Duration {
	if s == nil || s.Resources == nil || !s.Resources.Enabled {
		return base
	}
	_, lim, err := s.Resources.Current(ctx)
	if err != nil {
		return base
	}
	return resource.EffectiveTTL(base, 15*time.Second, lim)
}

func (s *Store) skipCacheSet(ctx context.Context) bool {
	if s == nil || s.Resources == nil || !s.Resources.Enabled {
		return false
	}
	_, lim, err := s.Resources.Current(ctx)
	if err != nil {
		return false
	}
	return lim.SkipRedisCacheSet
}

func (s *Store) Enabled() bool {
	return s != nil && s.rdb != nil
}

// Publish sends a message on a Redis pub/sub channel (best-effort, no-op when disabled).
func (s *Store) Publish(ctx context.Context, channel, message string) {
	if !s.Enabled() {
		return
	}
	_ = s.rdb.Publish(ctx, channel, message).Err()
}

// Subscribe invokes handler for every message on the channel until ctx is cancelled.
// It runs in its own goroutine and reconnects are handled by go-redis. No-op when disabled.
func (s *Store) Subscribe(ctx context.Context, channel string, handler func(message string)) {
	if !s.Enabled() || handler == nil {
		return
	}
	go func() {
		sub := s.rdb.Subscribe(ctx, channel)
		defer sub.Close() //nolint:errcheck
		ch := sub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				handler(msg.Payload)
			}
		}
	}()
}

func (s *Store) GetJSON(ctx context.Context, key string, dest any) (bool, error) {
	if !s.Enabled() {
		return false, nil
	}
	raw, err := s.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		_ = s.rdb.Del(ctx, key).Err()
		return false, fmt.Errorf("cache decode %q: %w", key, err)
	}
	return true, nil
}

func (s *Store) SetJSON(ctx context.Context, key string, v any, ttl time.Duration) error {
	if !s.Enabled() {
		return nil
	}
	if s.skipCacheSet(ctx) {
		return nil
	}
	return s.setJSONRaw(ctx, key, v, s.effectiveTTL(ctx, ttl))
}

func (s *Store) setJSONRaw(ctx context.Context, key string, v any, ttl time.Duration) error {
	if !s.Enabled() {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, key, b, ttl).Err()
}

func (s *Store) Delete(ctx context.Context, keys ...string) error {
	if !s.Enabled() || len(keys) == 0 {
		return nil
	}
	return s.rdb.Del(ctx, keys...).Err()
}

func (s *Store) GetBytes(ctx context.Context, key string) ([]byte, bool, error) {
	if !s.Enabled() {
		return nil, false, nil
	}
	raw, err := s.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return raw, true, nil
}

func (s *Store) SetBytes(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	if !s.Enabled() {
		return nil
	}
	if s.skipCacheSet(ctx) {
		return nil
	}
	return s.rdb.Set(ctx, key, data, s.effectiveTTL(ctx, ttl)).Err()
}

// DeleteByPrefix removes keys matching prefix* via SCAN (bounded iterations).
func (s *Store) DeleteByPrefix(ctx context.Context, prefix string) error {
	if !s.Enabled() || prefix == "" {
		return nil
	}
	var cursor uint64
	for i := 0; i < maxScanIterations; i++ {
		keys, next, err := s.rdb.Scan(ctx, cursor, prefix+"*", scanCount).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := s.rdb.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
	return fmt.Errorf("delete by prefix %q: scan iteration limit exceeded", prefix)
}

// GetOrLoadJSON returns cached value or runs load once for concurrent waiters.
func (s *Store) GetOrLoadJSON(
	ctx context.Context,
	key string,
	ttl time.Duration,
	dest any,
	load func(context.Context) (any, error),
) (bool, error) {
	if hit, err := s.GetJSON(ctx, key, dest); err != nil {
		return false, err
	} else if hit {
		return true, nil
	}
	if load == nil {
		return false, nil
	}
	raw, err, _ := s.sf.Do(key, func() (interface{}, error) {
		if hit, err := s.GetJSON(ctx, key, dest); err != nil {
			return nil, err
		} else if hit {
			return dest, nil
		}
		v, err := load(ctx)
		if err != nil {
			return nil, err
		}
		effTTL := s.effectiveTTL(ctx, ttl)
		if s.skipCacheSet(ctx) {
			return v, nil
		}
		if err := s.setJSONRaw(ctx, key, v, effTTL); err != nil {
			return nil, err
		}
		return v, nil
	})
	if err != nil {
		return false, err
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(b, dest); err != nil {
		return false, err
	}
	return false, nil
}
