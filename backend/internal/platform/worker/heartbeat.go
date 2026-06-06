package worker

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

const heartbeatKey = "worker:last_seen"
const heartbeatHashKey = "worker:heartbeats"

type Heartbeat struct {
	redis *redis.Client
	id    string
}

func NewHeartbeat(r *redis.Client) *Heartbeat {
	id := fmt.Sprintf("%s:%d", hostname(), os.Getpid())
	return &Heartbeat{redis: r, id: id}
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "worker"
	}
	return h
}

func (h *Heartbeat) Touch(ctx context.Context) error {
	if h.redis == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	pipe := h.redis.Pipeline()
	pipe.Set(ctx, heartbeatKey, now, 2*time.Minute)
	pipe.HSet(ctx, heartbeatHashKey, h.id, now)
	pipe.Expire(ctx, heartbeatHashKey, 2*time.Minute)
	_, err := pipe.Exec(ctx)
	return err
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

type WorkerHeartbeat struct {
	ID       string
	LastSeen string
}

func (h *Heartbeat) ActiveWorkers(ctx context.Context) ([]WorkerHeartbeat, bool) {
	if h.redis == nil {
		return nil, false
	}
	m, err := h.redis.HGetAll(ctx, heartbeatHashKey).Result()
	if err != nil || len(m) == 0 {
		return nil, false
	}
	out := make([]WorkerHeartbeat, 0, len(m))
	for id, ts := range m {
		out = append(out, WorkerHeartbeat{ID: id, LastSeen: ts})
	}
	return out, true
}

func (h *Heartbeat) ActiveWorkerCount(ctx context.Context) int {
	workers, ok := h.ActiveWorkers(ctx)
	if !ok {
		return 0
	}
	return len(workers)
}
