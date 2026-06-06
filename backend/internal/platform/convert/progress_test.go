package convert

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestProgressStoreCancel(t *testing.T) {
	t.Parallel()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewProgressStore(rdb)
	ctx := context.Background()
	jobID := "11111111-1111-4111-8111-111111111111"

	ok, err := store.IsCancelRequested(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected no cancel initially")
	}
	if err := store.RequestCancel(ctx, jobID); err != nil {
		t.Fatal(err)
	}
	ok, err = store.IsCancelRequested(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected cancel requested")
	}
	if err := store.ClearCancel(ctx, jobID); err != nil {
		t.Fatal(err)
	}
	ok, err = store.IsCancelRequested(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected cancel cleared")
	}
}
