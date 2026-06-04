package resource

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/anhtuanlc/mediahub/internal/observability"
	"github.com/redis/go-redis/v9"
)

// Collect builds a snapshot from local host metrics and optional Redis memory info.
func Collect(ctx context.Context, rdb *redis.Client) (*Snapshot, error) {
	host, err := observability.CollectHostStats(ctx)
	if err != nil {
		return nil, err
	}
	cpuIdle := clampPercent(100 - host.CPUPercent)
	ramIdle := clampPercent(100 - host.Memory.UsedPercent)
	redisIdle := 100.0
	if rdb != nil {
		if ri, err := redisMemoryIdle(ctx, rdb); err == nil {
			redisIdle = ri
		}
	}
	headroom := min3(cpuIdle, ramIdle, redisIdle)
	return &Snapshot{
		CPUIdlePercent:   round2(cpuIdle),
		RAMIdlePercent:   round2(ramIdle),
		RedisIdlePercent: round2(redisIdle),
		HeadroomPercent:  round2(headroom),
		Pressure:         round2(1 - headroom/100),
		CPUCount:         host.CPUCount,
		At:               time.Now().UTC(),
	}, nil
}

func redisMemoryIdle(ctx context.Context, rdb *redis.Client) (float64, error) {
	info, err := rdb.Info(ctx, "memory").Result()
	if err != nil {
		return 0, err
	}
	var used, max int64
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "used_memory:") {
			used, _ = strconv.ParseInt(strings.TrimPrefix(line, "used_memory:"), 10, 64)
		}
		if strings.HasPrefix(line, "maxmemory:") {
			max, _ = strconv.ParseInt(strings.TrimPrefix(line, "maxmemory:"), 10, 64)
		}
	}
	if max <= 0 {
		return 100, nil
	}
	usedPct := float64(used) / float64(max) * 100
	return clampPercent(100 - usedPct), nil
}

func clampPercent(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func min3(a, b, c float64) float64 {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
