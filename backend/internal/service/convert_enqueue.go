package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anhtuanlc/mediahub/internal/platform/logctx"
	internalworker "github.com/anhtuanlc/mediahub/internal/worker"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const asynqDefaultQueue = "default"

// ErrConvertQueueFull is returned when the Asynq convert queue exceeds its configured cap.
var ErrConvertQueueFull = fmt.Errorf("convert queue full")

type ConvertEnqueue struct {
	client    *asynq.Client
	inspector *asynq.Inspector
	maxDepth  int
}

func NewConvertEnqueue(redisAddr string, maxDepth int) *ConvertEnqueue {
	opt := asynq.RedisClientOpt{Addr: redisAddr}
	return &ConvertEnqueue{
		client:    asynq.NewClient(opt),
		inspector: asynq.NewInspector(opt),
		maxDepth:  maxDepth,
	}
}

func (e *ConvertEnqueue) Close() error {
	var first error
	if e.inspector != nil {
		if err := e.inspector.Close(); err != nil && first == nil {
			first = err
		}
	}
	if e.client != nil {
		if err := e.client.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// MaxDepth returns the configured queue cap (0 = unlimited).
func (e *ConvertEnqueue) MaxDepth() int {
	if e == nil {
		return 0
	}
	return e.maxDepth
}

// PendingDepth counts tasks waiting or running in the default Asynq queue.
func (e *ConvertEnqueue) PendingDepth(ctx context.Context) (int, error) {
	if e == nil || e.inspector == nil {
		return 0, nil
	}
	_ = ctx
	info, err := e.inspector.GetQueueInfo(asynqDefaultQueue)
	if err != nil {
		return 0, err
	}
	if info == nil {
		return 0, nil
	}
	return info.Pending + info.Active + info.Scheduled + info.Retry, nil
}

func (e *ConvertEnqueue) EnqueueConvert(ctx context.Context, jobPublicID, videoPublicID uuid.UUID, objectID int64, variants []string) error {
	if e.maxDepth > 0 {
		depth, err := e.PendingDepth(ctx)
		if err != nil {
			return err
		}
		if depth >= e.maxDepth {
			return ErrConvertQueueFull
		}
	}

	payload, err := json.Marshal(internalworker.ConvertPayload{
		JobPublicID:   jobPublicID.String(),
		VideoPublicID: videoPublicID.String(),
		ObjectID:      objectID,
		Variants:      variants,
		RequestID:     logctx.RequestIDFrom(ctx),
	})
	if err != nil {
		return err
	}
	task := asynq.NewTask(internalworker.TypeVideoConvert, payload)
	_, err = e.client.EnqueueContext(ctx, task)
	return err
}
