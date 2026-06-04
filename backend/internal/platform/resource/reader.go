package resource

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

// Reader loads resource snapshots (Redis first, then local collect).
type Reader struct {
	RDB     *redis.Client
	Policy  PolicyConfig
	Enabled bool
}

// Current returns the best available snapshot and derived limits.
func (r *Reader) Current(ctx context.Context) (*Snapshot, Limits, error) {
	if r == nil || !r.Enabled {
		return nil, conservativeLimits(r.safePolicy()), nil
	}
	if snap, err := loadFromRedis(ctx, r.RDB); err == nil && snap != nil {
		return snap, LimitsFromSnapshot(snap, r.Policy), nil
	}
	snap, err := Collect(ctx, r.RDB)
	if err != nil {
		return nil, conservativeLimits(r.Policy), err
	}
	return snap, LimitsFromSnapshot(snap, r.Policy), nil
}

func (r *Reader) safePolicy() PolicyConfig {
	if r == nil {
		return DefaultPolicyConfig(1, 2, 40, 15, 85)
	}
	return r.Policy
}

func loadFromRedis(ctx context.Context, rdb *redis.Client) (*Snapshot, error) {
	if rdb == nil {
		return nil, redis.Nil
	}
	raw, err := rdb.Get(ctx, RedisKeySnapshot).Bytes()
	if err != nil {
		return nil, err
	}
	var snap Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}
