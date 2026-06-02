package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/anhtuanlc/mediahub/internal/auth"
	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrSetupCompleted   = errors.New("setup already completed")
	ErrInvalidSetupToken = errors.New("invalid setup token")
	ErrInvalidEmail     = errors.New("invalid email")
)

type SetupStatus struct {
	SetupRequired bool `json:"setup_required"`
	SetupAllowed  bool `json:"setup_allowed"`
}

type CreateOwnerInput struct {
	Email      string
	Password   string
	SetupToken string
	IP         string
	UserAgent  string
}

type CreateOwnerResult struct {
	PublicID uuid.UUID `json:"public_id"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
}

type SetupService struct {
	users  *repository.UserRepository
	cfg    *config.Config
}

func NewSetupService(users *repository.UserRepository, cfg *config.Config) *SetupService {
	return &SetupService{users: users, cfg: cfg}
}

func (s *SetupService) GetStatus(ctx context.Context) (SetupStatus, error) {
	hasOwner, err := s.users.HasOwner(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	required := !hasOwner
	return SetupStatus{
		SetupRequired: required,
		SetupAllowed:  required,
	}, nil
}

func (s *SetupService) CreateOwner(ctx context.Context, in CreateOwnerInput) (*CreateOwnerResult, error) {
	hasOwner, err := s.users.HasOwner(ctx)
	if err != nil {
		return nil, err
	}
	if hasOwner {
		return nil, ErrSetupCompleted
	}

	if err := s.validateSetupToken(in.SetupToken); err != nil {
		return nil, err
	}

	email := strings.TrimSpace(strings.ToLower(in.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, ErrInvalidEmail
	}

	if err := auth.ValidatePasswordStrength(in.Password); err != nil {
		return nil, err
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	publicID := uuid.New()
	user, err := s.users.CreateOwner(ctx, email, hash, publicID, in.IP, in.UserAgent)
	if err != nil {
		if errors.Is(err, repository.ErrOwnerAlreadyExists) {
			return nil, ErrSetupCompleted
		}
		return nil, err
	}

	return &CreateOwnerResult{
		PublicID: user.PublicID,
		Email:    user.Email,
		Role:     user.Role,
	}, nil
}

func (s *SetupService) validateSetupToken(token string) error {
	if s.cfg.AppEnv != "production" && s.cfg.AppEnv != "staging" {
		return nil
	}
	if s.cfg.SetupToken == "" {
		return ErrInvalidSetupToken
	}
	if token != s.cfg.SetupToken {
		return ErrInvalidSetupToken
	}
	return nil
}
