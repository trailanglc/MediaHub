package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anhtuanlc/mediahub/internal/platform/logctx"
	internalworker "github.com/anhtuanlc/mediahub/internal/worker"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const asynqDownloadQueue = "download"

var ErrDownloadQueueFull = fmt.Errorf("download queue full")

type DownloadEnqueue struct {
	client    *asynq.Client
	inspector *asynq.Inspector
	settings  *SettingsService
	fallback  int
}

func NewDownloadEnqueue(redisAddr string, settings *SettingsService, fallbackMaxDepth int) *DownloadEnqueue {
	opt := asynq.RedisClientOpt{Addr: redisAddr}
	return &DownloadEnqueue{
		client:    asynq.NewClient(opt),
		inspector: asynq.NewInspector(opt),
		settings:  settings,
		fallback:  fallbackMaxDepth,
	}
}

func (e *DownloadEnqueue) Close() error {
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

func (e *DownloadEnqueue) maxDepth(ctx context.Context) int {
	if e != nil && e.settings != nil {
		if lim, err := e.settings.DownloadLimits(ctx); err == nil && lim.QueueMaxDepth > 0 {
			return lim.QueueMaxDepth
		}
	}
	if e != nil && e.fallback > 0 {
		return e.fallback
	}
	return 50
}

func (e *DownloadEnqueue) PendingDepth(ctx context.Context) (int, error) {
	_ = ctx
	if e == nil || e.inspector == nil {
		return 0, nil
	}
	info, err := e.inspector.GetQueueInfo(asynqDownloadQueue)
	if err != nil {
		if IsAsynqQueueMissing(err) {
			return 0, nil
		}
		return 0, err
	}
	return info.Pending + info.Active + info.Scheduled + info.Retry, nil
}

func (e *DownloadEnqueue) EnqueueFetch(ctx context.Context, jobPublicID uuid.UUID, maxAttempts int) error {
	if maxAttempts <= 0 {
		maxAttempts = defaultConvertMaxAttempts
	}
	maxDepth := e.maxDepth(ctx)
	if maxDepth > 0 {
		depth, err := e.PendingDepth(ctx)
		if err != nil {
			return err
		}
		if depth >= maxDepth {
			return ErrDownloadQueueFull
		}
	}
	payload, err := json.Marshal(internalworker.DownloadPayload{
		JobPublicID: jobPublicID.String(),
		RequestID:   logctx.RequestIDFrom(ctx),
	})
	if err != nil {
		return err
	}
	task := asynq.NewTask(internalworker.TypeDownloadFetch, payload)
	maxRetry := maxAttempts - 1
	if maxRetry < 0 {
		maxRetry = 0
	}
	// Unique task id per enqueue so retries don't collide with archived Asynq task ids.
	taskID := fmt.Sprintf("%s-%d", jobPublicID.String(), time.Now().UnixNano())
	_, err = e.client.EnqueueContext(ctx, task,
		asynq.Queue(asynqDownloadQueue),
		asynq.TaskID(taskID),
		asynq.MaxRetry(maxRetry),
	)
	return err
}

func (e *DownloadEnqueue) DeletePendingTaskByJobID(ctx context.Context, jobPublicID uuid.UUID) error {
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
		tasks, err := listFn(asynqDownloadQueue)
		if err != nil {
			if IsAsynqQueueMissing(err) {
				return nil
			}
			return err
		}
		for _, t := range tasks {
			var p internalworker.DownloadPayload
			if json.Unmarshal(t.Payload, &p) != nil || p.JobPublicID != target {
				continue
			}
			if err := e.inspector.DeleteTask(asynqDownloadQueue, t.ID); err != nil {
				return err
			}
			return nil
		}
	}
	return nil
}
