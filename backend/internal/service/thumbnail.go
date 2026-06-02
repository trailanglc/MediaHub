package service

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/anhtuanlc/mediahub/internal/mediautil"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/storage"
	"github.com/google/uuid"
)

const maxThumbnailSourceBytes = 25 * 1024 * 1024

type ThumbnailService struct {
	objects *repository.MediaObjectRepository
	store   storage.ObjectStorage
}

func NewThumbnailService(objects *repository.MediaObjectRepository, store storage.ObjectStorage) *ThumbnailService {
	return &ThumbnailService{objects: objects, store: store}
}

func (s *ThumbnailService) GenerateForObject(ctx context.Context, publicID uuid.UUID, sourceStorageKey string) error {
	if sourceStorageKey == "" {
		return fmt.Errorf("empty source key")
	}
	body, err := s.store.GetObject(ctx, sourceStorageKey)
	if err != nil {
		return err
	}
	defer body.Close()

	data, err := io.ReadAll(io.LimitReader(body, maxThumbnailSourceBytes+1))
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}
	if int64(len(data)) > maxThumbnailSourceBytes {
		return fmt.Errorf("source too large for thumbnail")
	}

	jpegBytes, err := mediautil.GenerateJPEGThumbnail(data, mediautil.ThumbnailMaxEdge)
	if err != nil {
		return err
	}

	thumbKey := storage.ThumbnailObjectKey(publicID)
	if err := s.store.PutObject(ctx, thumbKey, bytes.NewReader(jpegBytes), int64(len(jpegBytes)), "image/jpeg"); err != nil {
		return err
	}

	m, err := s.objects.GetByPublicID(ctx, publicID)
	if err != nil {
		return err
	}
	return s.objects.SetThumbnailKey(ctx, m.ID, thumbKey)
}
