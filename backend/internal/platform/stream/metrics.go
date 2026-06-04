package stream

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	keyRequestsPrefix  = "stream:req:"
	keyBandwidthPrefix = "stream:bw:"
	analyticsKey       = "stream:analytics:total"

	// Per-day counters expire so Redis does not grow without bound.
	dailyMetricsRetention = 90 * 24 * time.Hour
)

type Metrics struct {
	redis *redis.Client
}

func NewMetrics(r *redis.Client) *Metrics {
	return &Metrics{redis: r}
}

func (m *Metrics) RecordAccess(ctx context.Context, videoPublicID string, bytes int64) error {
	if m.redis == nil {
		return nil
	}
	day := time.Now().UTC().Format("2006-01-02")
	reqVideo := keyRequestsPrefix + videoPublicID + ":" + day
	reqGlobal := keyRequestsPrefix + "global:" + day
	bwVideo := keyBandwidthPrefix + videoPublicID + ":" + day
	bwGlobal := keyBandwidthPrefix + "global:" + day

	pipe := m.redis.Pipeline()
	pipe.Incr(ctx, reqVideo)
	pipe.Expire(ctx, reqVideo, dailyMetricsRetention)
	pipe.Incr(ctx, reqGlobal)
	pipe.Expire(ctx, reqGlobal, dailyMetricsRetention)
	if bytes > 0 {
		pipe.IncrBy(ctx, bwVideo, bytes)
		pipe.Expire(ctx, bwVideo, dailyMetricsRetention)
		pipe.IncrBy(ctx, bwGlobal, bytes)
		pipe.Expire(ctx, bwGlobal, dailyMetricsRetention)
	}
	pipe.HIncrBy(ctx, analyticsKey, "requests", 1)
	if bytes > 0 {
		pipe.HIncrBy(ctx, analyticsKey, "bytes", bytes)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (m *Metrics) AnalyticsSnapshot(ctx context.Context) (map[string]int64, error) {
	out := map[string]int64{"requests_today": 0, "bytes_today": 0, "requests_total": 0, "bytes_total": 0}
	if m.redis == nil {
		return out, nil
	}
	day := time.Now().UTC().Format("2006-01-02")
	req, _ := m.redis.Get(ctx, keyRequestsPrefix+"global:"+day).Int64()
	bw, _ := m.redis.Get(ctx, keyBandwidthPrefix+"global:"+day).Int64()
	out["requests_today"] = req
	out["bytes_today"] = bw
	tot, err := m.redis.HGetAll(ctx, analyticsKey).Result()
	if err != nil {
		return out, err
	}
	for k, v := range tot {
		var n int64
		fmt.Sscanf(v, "%d", &n)
		if k == "requests" {
			out["requests_total"] = n
		}
		if k == "bytes" {
			out["bytes_total"] = n
		}
	}
	return out, nil
}

type RateLimiter struct {
	redis      *redis.Client
	perMin     int
	failClosed bool
}

func NewRateLimiter(r *redis.Client, perMin int, failClosed bool) *RateLimiter {
	if perMin <= 0 {
		perMin = 120
	}
	return &RateLimiter{redis: r, perMin: perMin, failClosed: failClosed}
}

func (l *RateLimiter) Allow(ctx context.Context, ip, videoID string) (bool, error) {
	if l.redis == nil {
		return true, nil
	}
	key := fmt.Sprintf("stream:rl:%s:%s:%d", ip, videoID, time.Now().Unix()/60)
	n, err := l.redis.Incr(ctx, key).Result()
	if err != nil {
		if l.failClosed {
			return false, err
		}
		return true, err
	}
	if n == 1 {
		_ = l.redis.Expire(ctx, key, 2*time.Minute).Err()
	}
	return n <= int64(l.perMin), nil
}
