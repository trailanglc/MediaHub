package worker

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const heartbeatKey = "worker:last_seen"

type Heartbeat struct {
	redis *redis.Client
}

func NewHeartbeat(r *redis.Client) *Heartbeat {
	return &Heartbeat{redis: r}
}

func (h *Heartbeat) Touch(ctx context.Context) error {
	if h.redis == nil {
		return nil
	}
	return h.redis.Set(ctx, heartbeatKey, time.Now().UTC().Format(time.RFC3339), 2*time.Minute).Err()
}

func (h *Heartbeat) LastSeen(ctx context.Context) (string, bool) {
	if h.redis == nil {
		return "", false
	}
	v, err := h.redis.Get(ctx, heartbeatKey).Result()
	if err != nil {
		return "", false
	}
	return v, true
}
