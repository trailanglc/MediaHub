package platform

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type UploadRateLimiter struct {
	rdb          *redis.Client
	maxPerMinute int
}

func NewUploadRateLimiter(rdb *redis.Client, maxPerMinute int) *UploadRateLimiter {
	if maxPerMinute <= 0 {
		maxPerMinute = 60
	}
	return &UploadRateLimiter{rdb: rdb, maxPerMinute: maxPerMinute}
}

func (l *UploadRateLimiter) AllowInit(ctx context.Context, userID int64) (bool, error) {
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
