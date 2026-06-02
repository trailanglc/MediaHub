package worker

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

type ConvertHandler struct {
	Log *zap.Logger
}

func (h *ConvertHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload map[string]any
	if len(t.Payload()) > 0 {
		_ = json.Unmarshal(t.Payload(), &payload)
	}
	h.Log.Info("video convert task received (stub)", zap.String("type", t.Type()))
	return nil
}
