package downloadprog

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "download:progress:"
const cancelKeyPrefix = "download:cancel:"
const eventChannelPrefix = "download:events:"
const publishThrottlePrefix = "download:pubthrottle:"
const persistThrottlePrefix = "download:dbthrottle:"

type Progress struct {
	Stage      string `json:"stage"`
	Percent    int    `json:"percent"`
	BytesDone  int64  `json:"bytes_done,omitempty"`
	BytesTotal int64  `json:"bytes_total,omitempty"`
}

// JobEvent is pushed over Redis pub/sub for SSE clients.
type JobEvent struct {
	PublicID      string  `json:"public_id"`
	CreatedBy     int64   `json:"created_by"`
	Status        string  `json:"status"`
	Kind          string  `json:"kind,omitempty"`
	SourceURL     string  `json:"source_url,omitempty"`
	Title         *string `json:"title,omitempty"`
	ThumbnailURL  *string `json:"thumbnail_url,omitempty"`
	ProgressPct   int     `json:"progress_pct"`
	ProgressStage string  `json:"progress_stage,omitempty"`
	BytesDone     int64   `json:"bytes_done"`
	BytesTotal    int64   `json:"bytes_total"`
	VideoPublicID *string `json:"video_public_id,omitempty"`
	LastError     *string `json:"last_error,omitempty"`
	CreatedAt     string  `json:"created_at,omitempty"`
	UpdatedAt     string  `json:"updated_at,omitempty"`
}

type ProgressStore struct {
	redis *redis.Client
}

func NewProgressStore(r *redis.Client) *ProgressStore {
	return &ProgressStore{redis: r}
}

func (s *ProgressStore) key(jobPublicID string) string {
	return keyPrefix + jobPublicID
}

func (s *ProgressStore) Set(ctx context.Context, jobPublicID string, p Progress) error {
	if s.redis == nil {
		return nil
	}
	if p.Percent < 0 {
		p.Percent = 0
	}
	if p.Percent > 100 {
		p.Percent = 100
	}
	b, _ := json.Marshal(p)
	return s.redis.Set(ctx, s.key(jobPublicID), b, 24*time.Hour).Err()
}

func (s *ProgressStore) Get(ctx context.Context, jobPublicID string) (*Progress, error) {
	if s.redis == nil {
		return nil, nil
	}
	raw, err := s.redis.Get(ctx, s.key(jobPublicID)).Bytes()
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

func (s *ProgressStore) Clear(ctx context.Context, jobPublicID string) error {
	if s.redis == nil {
		return nil
	}
	return s.redis.Del(ctx, s.key(jobPublicID), persistThrottlePrefix+jobPublicID, publishThrottlePrefix+jobPublicID).Err()
}

func (s *ProgressStore) cancelKey(jobPublicID string) string {
	return cancelKeyPrefix + jobPublicID
}

func (s *ProgressStore) RequestCancel(ctx context.Context, jobPublicID string) error {
	if s.redis == nil {
		return nil
	}
	return s.redis.Set(ctx, s.cancelKey(jobPublicID), "1", 24*time.Hour).Err()
}

func (s *ProgressStore) IsCancelRequested(ctx context.Context, jobPublicID string) (bool, error) {
	if s.redis == nil {
		return false, nil
	}
	n, err := s.redis.Exists(ctx, s.cancelKey(jobPublicID)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *ProgressStore) ClearCancel(ctx context.Context, jobPublicID string) error {
	if s.redis == nil {
		return nil
	}
	return s.redis.Del(ctx, s.cancelKey(jobPublicID)).Err()
}

func (s *ProgressStore) eventChannel(userID int64) string {
	return fmt.Sprintf("%s%d", eventChannelPrefix, userID)
}

// ShouldPersistDB returns true about once per second per job (or when force).
// Keeps Postgres writes light so progress callbacks don't slow the download.
func (s *ProgressStore) ShouldPersistDB(ctx context.Context, jobPublicID string, force bool) bool {
	if force {
		return true
	}
	if s.redis == nil {
		return true
	}
	ok, err := s.redis.SetNX(ctx, persistThrottlePrefix+jobPublicID, "1", time.Second).Result()
	if err != nil {
		return true
	}
	return ok
}

// Publish pushes a job event to the owning user's Redis channel.
// Always refreshes the Redis progress snapshot so reload/List sees live bytes.
// Non-terminal running updates are SSE-throttled (~750ms) to avoid flooding clients.
func (s *ProgressStore) Publish(ctx context.Context, ev JobEvent, force bool) error {
	if s.redis == nil {
		return nil
	}
	_ = s.Set(ctx, ev.PublicID, Progress{
		Stage:      ev.ProgressStage,
		Percent:    ev.ProgressPct,
		BytesDone:  ev.BytesDone,
		BytesTotal: ev.BytesTotal,
	})
	if ev.CreatedBy == 0 {
		return nil
	}
	terminal := ev.Status == "succeeded" || ev.Status == "failed" || ev.Status == "cancelled" ||
		ev.Status == "pending" || ev.Status == "deleted" || force
	if !terminal {
		throttleKey := publishThrottlePrefix + ev.PublicID
		ok, err := s.redis.SetNX(ctx, throttleKey, "1", 750*time.Millisecond).Result()
		if err == nil && !ok {
			return nil
		}
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return s.redis.Publish(ctx, s.eventChannel(ev.CreatedBy), b).Err()
}

// Subscribe returns a pubsub for the user's download events. Caller must Close.
func (s *ProgressStore) Subscribe(ctx context.Context, userID int64) *redis.PubSub {
	if s.redis == nil {
		return nil
	}
	return s.redis.Subscribe(ctx, s.eventChannel(userID))
}
