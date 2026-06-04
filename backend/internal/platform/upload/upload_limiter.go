package upload

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	rdb          *redis.Client
	maxPerMinute int
}

func NewRateLimiter(rdb *redis.Client, maxPerMinute int) *RateLimiter {
	if maxPerMinute <= 0 {
		maxPerMinute = 60
	}
	return &RateLimiter{rdb: rdb, maxPerMinute: maxPerMinute}
}

func (l *RateLimiter) AllowInit(ctx context.Context, userID int64) (bool, error) {
	if l.rdb == nil {
		return true, nil
	}
	bucket := time.Now().UTC().Unix() / 60
	key := fmt.Sprintf("upload:init:%d:%d", userID, bucket)
	n, err := l.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		_ = l.rdb.Expire(ctx, key, 2*time.Minute).Err()
	}
	return n <= int64(l.maxPerMinute), nil
}
