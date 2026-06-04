package stream

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRateLimiter_failClosed(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rl := NewRateLimiter(rdb, 10, true)
	mr.SetError("down")
	ok, err := rl.Allow(context.Background(), "1.2.3.4", "vid")
	if err == nil || ok {
		t.Fatalf("want deny on redis error, ok=%v err=%v", ok, err)
	}
}
