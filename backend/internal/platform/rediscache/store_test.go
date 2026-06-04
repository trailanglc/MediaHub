package rediscache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestStore_SetGetDelete(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewStore(rdb)
	ctx := context.Background()

	type payload struct {
		N int `json:"n"`
	}
	if err := store.SetJSON(ctx, "cache:test:1", payload{N: 42}, time.Minute); err != nil {
		t.Fatal(err)
	}
	var got payload
	hit, err := store.GetJSON(ctx, "cache:test:1", &got)
	if err != nil || !hit || got.N != 42 {
		t.Fatalf("get: hit=%v got=%+v err=%v", hit, got, err)
	}
	if err := store.Delete(ctx, "cache:test:1"); err != nil {
		t.Fatal(err)
	}
	hit, err = store.GetJSON(ctx, "cache:test:1", &got)
	if err != nil || hit {
		t.Fatalf("expected miss after delete, hit=%v err=%v", hit, err)
	}
}

func TestStore_DeleteByPrefix(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewStore(rdb)
	ctx := context.Background()

	_ = store.SetJSON(ctx, PrefixStreamVideo+"a", map[string]int{"x": 1}, time.Minute)
	_ = store.SetJSON(ctx, PrefixStreamVideo+"b", map[string]int{"x": 2}, time.Minute)
	if err := store.DeleteByPrefix(ctx, PrefixStreamVideo); err != nil {
		t.Fatal(err)
	}
	var v map[string]int
	if hit, _ := store.GetJSON(ctx, PrefixStreamVideo+"a", &v); hit {
		t.Fatal("expected prefix delete to remove stream keys")
	}
}

func TestKeyStreamVideo(t *testing.T) {
	if KeyStreamVideo("vid") != PrefixStreamVideo+"vid" {
		t.Fatal("key format")
	}
}
