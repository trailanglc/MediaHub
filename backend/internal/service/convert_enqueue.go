package service

import (
	"context"
	"encoding/json"

	internalworker "github.com/anhtuanlc/mediahub/internal/worker"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

type ConvertEnqueue struct {
	client *asynq.Client
}

func NewConvertEnqueue(redisAddr string) *ConvertEnqueue {
	return &ConvertEnqueue{
		client: asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr}),
	}
}

func (e *ConvertEnqueue) Close() error {
	if e.client == nil {
		return nil
	}
	return e.client.Close()
}

func (e *ConvertEnqueue) EnqueueConvert(ctx context.Context, jobPublicID, videoPublicID uuid.UUID, objectID int64, variants []string) error {
	payload, err := json.Marshal(internalworker.ConvertPayload{
		JobPublicID:   jobPublicID.String(),
		VideoPublicID: videoPublicID.String(),
		ObjectID:      objectID,
		Variants:      variants,
	})
	if err != nil {
		return err
	}
	task := asynq.NewTask(internalworker.TypeVideoConvert, payload)
	_, err = e.client.EnqueueContext(ctx, task)
	return err
}
