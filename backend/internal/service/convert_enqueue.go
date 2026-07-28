package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/anhtuanlc/mediahub/internal/platform/logctx"
	internalworker "github.com/anhtuanlc/mediahub/internal/worker"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const asynqDefaultQueue = "default"

const defaultConvertMaxAttempts = 3

// ErrConvertQueueFull is returned when the Asynq convert queue exceeds its configured cap.
var ErrConvertQueueFull = fmt.Errorf("convert queue full")

// ErrConvertQueuePaused is returned when the convert queue is paused.
var ErrConvertQueuePaused = fmt.Errorf("convert queue paused")

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

// IsAsynqQueueMissing treats an empty/missing convert queue as idle (not an API error).
// GetQueueInfo returns an unwrapped NOT_FOUND; Pause/List wrap asynq.ErrQueueNotFound.
func IsAsynqQueueMissing(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, asynq.ErrQueueNotFound) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "does not exist") || strings.HasPrefix(msg, "NOT_FOUND:")
}

func (e *ConvertEnqueue) queueInfo() (*asynq.QueueInfo, error) {
	if e == nil || e.inspector == nil {
		return nil, nil
	}
	info, err := e.inspector.GetQueueInfo(asynqDefaultQueue)
	if err != nil {
		if IsAsynqQueueMissing(err) {
			return nil, nil
		}
		return nil, err
	}
	return info, nil
}

// PendingDepth counts tasks waiting or running in the default Asynq queue.
func (e *ConvertEnqueue) PendingDepth(ctx context.Context) (int, error) {
	_ = ctx
	info, err := e.queueInfo()
	if err != nil {
		return 0, err
	}
	if info == nil {
		return 0, nil
	}
	return info.Pending + info.Active + info.Scheduled + info.Retry, nil
}

// IsQueuePaused reports whether the default convert queue is paused in Asynq.
func (e *ConvertEnqueue) IsQueuePaused(ctx context.Context) (bool, error) {
	_ = ctx
	info, err := e.queueInfo()
	if err != nil {
		return false, err
	}
	if info == nil {
		return false, nil
	}
	return info.Paused, nil
}

func (e *ConvertEnqueue) PauseQueue(ctx context.Context) error {
	if e == nil || e.inspector == nil {
		return fmt.Errorf("convert enqueue not configured")
	}
	_ = ctx
	err := e.inspector.PauseQueue(asynqDefaultQueue)
	if IsAsynqQueueMissing(err) {
		return nil
	}
	return err
}

func (e *ConvertEnqueue) ResumeQueue(ctx context.Context) error {
	if e == nil || e.inspector == nil {
		return fmt.Errorf("convert enqueue not configured")
	}
	_ = ctx
	err := e.inspector.UnpauseQueue(asynqDefaultQueue)
	if IsAsynqQueueMissing(err) {
		return nil
	}
	return err
}

func (e *ConvertEnqueue) EnqueueConvert(ctx context.Context, jobPublicID, videoPublicID uuid.UUID, objectID int64, variants []string, maxAttempts int) error {
	if maxAttempts <= 0 {
		maxAttempts = defaultConvertMaxAttempts
	}
	if paused, err := e.IsQueuePaused(ctx); err != nil {
		return err
	} else if paused {
		return ErrConvertQueuePaused
	}
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
	maxRetry := maxAttempts - 1
	if maxRetry < 0 {
		maxRetry = 0
	}
	_, err = e.client.EnqueueContext(ctx, task,
		asynq.Queue(asynqDefaultQueue),
		asynq.TaskID(jobPublicID.String()),
		asynq.MaxRetry(maxRetry),
	)
	return err
}

// DeletePendingTaskByJobID removes a queued convert task matching job_public_id.
func (e *ConvertEnqueue) DeletePendingTaskByJobID(ctx context.Context, jobPublicID uuid.UUID) error {
	if e == nil || e.inspector == nil {
		return nil
	}
	_ = ctx
	target := jobPublicID.String()
	listFns := []func(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error){
		e.inspector.ListPendingTasks,
		e.inspector.ListScheduledTasks,
		e.inspector.ListRetryTasks,
	}
	for _, listFn := range listFns {
		tasks, err := listFn(asynqDefaultQueue)
		if err != nil {
			if IsAsynqQueueMissing(err) {
				return nil
			}
			return err
		}
		for _, t := range tasks {
			var p internalworker.ConvertPayload
			if json.Unmarshal(t.Payload, &p) != nil || p.JobPublicID != target {
				continue
			}
			if err := e.inspector.DeleteTask(asynqDefaultQueue, t.ID); err != nil {
				return err
			}
			return nil
		}
	}
	return nil
}
