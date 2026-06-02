package platform

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// SessionInvalidation marks all access tokens issued before a timestamp as invalid for a user.
type SessionInvalidation struct {
	rdb *redis.Client
}

func NewSessionInvalidation(rdb *redis.Client) *SessionInvalidation {
	return &SessionInvalidation{rdb: rdb}
}

func sessionInvalidateKey(userID int64) string {
	return fmt.Sprintf("user:auth_invalidate_after:%d", userID)
}

func (s *SessionInvalidation) InvalidateUser(ctx context.Context, userID int64) error {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	if err := s.rdb.Set(ctx, sessionInvalidateKey(userID), ts, 0).Err(); err != nil {
		return fmt.Errorf("invalidate user sessions: %w", err)
	}
	return nil
}

func (s *SessionInvalidation) IssuedAtValid(ctx context.Context, userID int64, issuedAt time.Time) (bool, error) {
	val, err := s.rdb.Get(ctx, sessionInvalidateKey(userID)).Result()
	if err == redis.Nil {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("check session invalidation: %w", err)
	}
	cutoff, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return false, fmt.Errorf("parse invalidation timestamp: %w", err)
	}
	if issuedAt.Unix() < cutoff {
		return false, nil
	}
	return true, nil
}
