package stream

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func benchRedis(b *testing.B) *redis.Client {
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(mr.Close)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func BenchmarkRateLimiterAllow(b *testing.B) {
	rl := NewRateLimiter(benchRedis(b), 1_000_000, false)
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = rl.Allow(ctx, "1.2.3.4", "vid")
	}
}

func BenchmarkRecordAccess(b *testing.B) {
	m := NewMetrics(benchRedis(b))
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = m.RecordAccess(ctx, "550e8400-e29b-41d4-a716-446655440000", 1500)
	}
}
