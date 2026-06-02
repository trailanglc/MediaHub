package platform

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// TTLCache stores a value with TTL. Many concurrent callers share one refresh
// when the entry expires (singleflight).
type TTLCache[T any] struct {
	ttl time.Duration
	mu  sync.RWMutex
	at  time.Time
	val T
	ok  bool
	sf  singleflight.Group
}

func NewTTLCache[T any](ttl time.Duration) *TTLCache[T] {
	return &TTLCache[T]{ttl: ttl}
}

func (c *TTLCache[T]) TTL() time.Duration {
	return c.ttl
}

func (c *TTLCache[T]) CachedAt() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.ok {
		return time.Time{}
	}
	return c.at
}

func (c *TTLCache[T]) peek() (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var zero T
	if !c.ok || time.Since(c.at) >= c.ttl {
		return zero, false
	}
	return c.val, true
}

func (c *TTLCache[T]) store(v T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.val = v
	c.at = time.Now()
	c.ok = true
}

// GetOrCompute returns cached data or runs compute once for concurrent waiters.
// On compute error, returns the previous cached value if any (stale-while-error).
func (c *TTLCache[T]) GetOrCompute(
	ctx context.Context,
	compute func(context.Context) (T, error),
) (T, error) {
	var zero T

	if v, hit := c.peek(); hit {
		return v, nil
	}

	raw, err, _ := c.sf.Do("refresh", func() (interface{}, error) {
		if v, hit := c.peek(); hit {
			return v, nil
		}
		v, err := compute(ctx)
		if err != nil {
			return nil, err
		}
		c.store(v)
		return v, nil
	})
	if err != nil {
		if v, hit := c.peek(); hit {
			return v, nil
		}
		return zero, err
	}
	return raw.(T), nil
}
