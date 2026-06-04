package convert

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "convert:progress:"

// Progress is cached in Redis while a convert job runs.
type Progress struct {
	Stage   string `json:"stage"`
	Percent int    `json:"percent"`
}

type ProgressStore struct {
	redis *redis.Client
}

func NewProgressStore(r *redis.Client) *ProgressStore {
	return &ProgressStore{redis: r}
}

func (s *ProgressStore) key(videoPublicID string) string {
	return keyPrefix + videoPublicID
}

func (s *ProgressStore) Set(ctx context.Context, videoPublicID, stage string, percent int) error {
	if s.redis == nil {
		return nil
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	b, _ := json.Marshal(Progress{Stage: stage, Percent: percent})
	return s.redis.Set(ctx, s.key(videoPublicID), b, 24*time.Hour).Err()
}

func (s *ProgressStore) Get(ctx context.Context, videoPublicID string) (*Progress, error) {
	if s.redis == nil {
		return nil, nil
	}
	raw, err := s.redis.Get(ctx, s.key(videoPublicID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p Progress
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("parse progress: %w", err)
	}
	return &p, nil
}

func (s *ProgressStore) Clear(ctx context.Context, videoPublicID string) error {
	if s.redis == nil {
		return nil
	}
	return s.redis.Del(ctx, s.key(videoPublicID)).Err()
}
