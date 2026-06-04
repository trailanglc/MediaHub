package integrationquota

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter caps integration API usage per API key (never applied to /stream segments).
type Limiter struct {
	redis           *redis.Client
	uploadInitPerMin int
	convertPerHour   int
}

func NewLimiter(r *redis.Client, uploadInitPerMin, convertPerHour int) *Limiter {
	if uploadInitPerMin <= 0 {
		uploadInitPerMin = 120
	}
	if convertPerHour <= 0 {
		convertPerHour = 60
	}
	return &Limiter{redis: r, uploadInitPerMin: uploadInitPerMin, convertPerHour: convertPerHour}
}

func (l *Limiter) AllowUploadInit(ctx context.Context, apiKeyID int64) (bool, error) {
	return l.allow(ctx, fmt.Sprintf("apikey:upload:init:%d:%d", apiKeyID, time.Now().Unix()/60), l.uploadInitPerMin, 2*time.Minute)
}

func (l *Limiter) AllowConvert(ctx context.Context, apiKeyID int64) (bool, error) {
	return l.allow(ctx, fmt.Sprintf("apikey:convert:%d:%d", apiKeyID, time.Now().Unix()/3600), l.convertPerHour, 2*time.Hour)
}

func (l *Limiter) allow(ctx context.Context, key string, limit int, ttl time.Duration) (bool, error) {
	if l == nil || l.redis == nil {
		return true, nil
	}
	n, err := l.redis.Incr(ctx, key).Result()
	if err != nil {
		return true, err // fail open for integration throughput
	}
	if n == 1 {
		_ = l.redis.Expire(ctx, key, ttl).Err()
	}
	return n <= int64(limit), nil
}
