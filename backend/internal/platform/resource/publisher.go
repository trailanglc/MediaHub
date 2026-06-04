package resource

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// Publish stores the snapshot in Redis for other processes.
func Publish(ctx context.Context, rdb *redis.Client, snap *Snapshot, ttl time.Duration) error {
	if rdb == nil || snap == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = defaultSnapshotTTL
	}
	b, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	return rdb.Set(ctx, RedisKeySnapshot, b, ttl).Err()
}
