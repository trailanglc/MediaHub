package session

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestLoginRateLimiter_failClosed(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	limiter := NewLoginRateLimiter(rdb, 3, time.Minute, true)

	mr.SetError("simulated redis down")
	locked, err := limiter.IsLocked(context.Background(), "a@b.com", "1.2.3.4")
	if err == nil || !locked {
		t.Fatalf("expected locked with error, got locked=%v err=%v", locked, err)
	}
}

func TestLoginRateLimiter_failOpen(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	limiter := NewLoginRateLimiter(rdb, 3, time.Minute, false)

	mr.SetError("simulated redis down")
	locked, err := limiter.IsLocked(context.Background(), "a@b.com", "1.2.3.4")
	if err == nil || locked {
		t.Fatalf("expected not locked with error, got locked=%v err=%v", locked, err)
	}
}
