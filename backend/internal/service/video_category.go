package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrVideoCategoryNotFound = errors.New("video category not found")
	ErrVideoCategoryExists   = errors.New("video category already exists")
	ErrVideoCategoryInvalid  = errors.New("invalid video category name")
)

type VideoCategoryDTO struct {
	PublicID   string    `json:"public_id"`
	Name       string    `json:"name"`
	SortOrder  int       `json:"sort_order"`
	VideoCount int64     `json:"video_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type VideoCategoryService struct {
	categories *repository.VideoCategoryRepository
}

func NewVideoCategoryService(categories *repository.VideoCategoryRepository) *VideoCategoryService {
	return &VideoCategoryService{categories: categories}
}

func categoryToDTO(c *repository.VideoCategory) VideoCategoryDTO {
	return VideoCategoryDTO{
		PublicID:   c.PublicID.String(),
		Name:       c.Name,
		SortOrder:  c.SortOrder,
		VideoCount: c.VideoCount,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
	}
}

func (s *VideoCategoryService) List(ctx context.Context) ([]VideoCategoryDTO, error) {
	list, err := s.categories.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]VideoCategoryDTO, 0, len(list))
	for i := range list {
		out = append(out, categoryToDTO(&list[i]))
	}
	return out, nil
}

func (s *VideoCategoryService) Get(ctx context.Context, publicID uuid.UUID) (*VideoCategoryDTO, error) {
	c, err := s.categories.GetByPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, repository.ErrVideoCategoryNotFound) {
			return nil, ErrVideoCategoryNotFound
		}
		return nil, err
	}
	dto := categoryToDTO(c)
	return &dto, nil
}

type CreateVideoCategoryInput struct {
	Name string `json:"name"`
}

func (s *VideoCategoryService) Create(ctx context.Context, userID int64, in CreateVideoCategoryInput) (*VideoCategoryDTO, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > 200 {
		return nil, ErrVideoCategoryInvalid
	}
	uid := userID
	c, err := s.categories.Create(ctx, name, &uid)
	if err != nil {
		if errors.Is(err, repository.ErrVideoCategoryExists) {
			return nil, ErrVideoCategoryExists
		}
		return nil, err
	}
	dto := categoryToDTO(c)
	return &dto, nil
}

type UpdateVideoCategoryInput struct {
	Name string `json:"name"`
}

func (s *VideoCategoryService) Update(ctx context.Context, publicID uuid.UUID, in UpdateVideoCategoryInput) (*VideoCategoryDTO, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > 200 {
		return nil, ErrVideoCategoryInvalid
	}
	c, err := s.categories.GetByPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, repository.ErrVideoCategoryNotFound) {
			return nil, ErrVideoCategoryNotFound
		}
		return nil, err
	}
	updated, err := s.categories.UpdateName(ctx, c.ID, name)
	if err != nil {
		if errors.Is(err, repository.ErrVideoCategoryExists) {
			return nil, ErrVideoCategoryExists
		}
		if errors.Is(err, repository.ErrVideoCategoryNotFound) {
			return nil, ErrVideoCategoryNotFound
		}
		return nil, err
	}
	dto := categoryToDTO(updated)
	return &dto, nil
}

func (s *VideoCategoryService) Delete(ctx context.Context, publicID uuid.UUID) error {
	c, err := s.categories.GetByPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, repository.ErrVideoCategoryNotFound) {
			return ErrVideoCategoryNotFound
		}
		return err
	}
	if err := s.categories.Delete(ctx, c.ID); err != nil {
		if errors.Is(err, repository.ErrVideoCategoryNotFound) {
			return ErrVideoCategoryNotFound
		}
		return err
	}
	return nil
}
