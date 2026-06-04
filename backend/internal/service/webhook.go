package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/anhtuanlc/mediahub/internal/platform/webhook"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/google/uuid"
)

type WebhookDTO struct {
	PublicID  string   `json:"public_id"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	Status    string   `json:"status"`
	CreatedAt string   `json:"created_at"`
}

type WebhookService struct {
	repo *repository.WebhookRepository
}

func NewWebhookService(repo *repository.WebhookRepository) *WebhookService {
	return &WebhookService{repo: repo}
}

func toWebhookDTO(w *repository.Webhook) WebhookDTO {
	return WebhookDTO{
		PublicID:  w.PublicID.String(),
		URL:       w.URL,
		Events:    w.Events,
		Status:    w.Status,
		CreatedAt: w.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

type CreateWebhookInput struct {
	URL    string
	Events []string
}

type CreateWebhookResult struct {
	Webhook WebhookDTO `json:"webhook"`
	Secret  string     `json:"signing_secret"`
}

func (s *WebhookService) List(ctx context.Context) ([]WebhookDTO, error) {
	list, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]WebhookDTO, 0, len(list))
	for i := range list {
		out = append(out, toWebhookDTO(&list[i]))
	}
	return out, nil
}

func (s *WebhookService) Create(ctx context.Context, createdBy int64, in CreateWebhookInput) (*CreateWebhookResult, error) {
	u, err := url.Parse(strings.TrimSpace(in.URL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid webhook url")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, fmt.Errorf("webhook url must be http or https")
	}
	events := normalizeWebhookEvents(in.Events)
	if len(events) == 0 {
		return nil, fmt.Errorf("at least one event is required")
	}
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, err
	}
	secret := "whsec_" + base64.RawURLEncoding.EncodeToString(secretBytes)
	w, err := s.repo.Create(ctx, uuid.New(), u.String(), secret, events, createdBy)
	if err != nil {
		return nil, err
	}
	dto := toWebhookDTO(w)
	return &CreateWebhookResult{Webhook: dto, Secret: secret}, nil
}

func (s *WebhookService) Delete(ctx context.Context, publicID uuid.UUID) error {
	return s.repo.Delete(ctx, publicID)
}

func normalizeWebhookEvents(events []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, e := range events {
		e = strings.TrimSpace(e)
		if e == "" || !webhook.ValidEvent(e) {
			continue
		}
		if _, ok := seen[e]; ok {
			continue
		}
		seen[e] = struct{}{}
		out = append(out, e)
	}
	return out
}
