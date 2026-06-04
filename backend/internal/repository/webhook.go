package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrWebhookNotFound = errors.New("webhook not found")

type Webhook struct {
	ID             int64
	PublicID       uuid.UUID
	URL            string
	SigningSecret  string
	Events         []string
	Status         string
	CreatedBy      *int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type WebhookRepository struct {
	pool *pgxpool.Pool
}

func NewWebhookRepository(pool *pgxpool.Pool) *WebhookRepository {
	return &WebhookRepository{pool: pool}
}

func scanWebhook(row pgx.Row) (*Webhook, error) {
	var w Webhook
	var eventsJSON []byte
	err := row.Scan(
		&w.ID, &w.PublicID, &w.URL, &w.SigningSecret, &eventsJSON,
		&w.Status, &w.CreatedBy, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWebhookNotFound
		}
		return nil, err
	}
	_ = json.Unmarshal(eventsJSON, &w.Events)
	if w.Events == nil {
		w.Events = []string{}
	}
	return &w, nil
}

func (r *WebhookRepository) Create(ctx context.Context, publicID uuid.UUID, url, secret string, events []string, createdBy int64) (*Webhook, error) {
	eventsJSON, _ := json.Marshal(events)
	row := r.pool.QueryRow(ctx, `
		INSERT INTO webhooks (public_id, url, signing_secret, events, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, public_id, url, signing_secret, events, status, created_by, created_at, updated_at
	`, publicID, url, secret, eventsJSON, createdBy)
	return scanWebhook(row)
}

func (r *WebhookRepository) List(ctx context.Context) ([]Webhook, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, public_id, url, signing_secret, events, status, created_by, created_at, updated_at
		FROM webhooks ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Webhook
	for rows.Next() {
		w, err := scanWebhook(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *w)
	}
	return list, rows.Err()
}

func (r *WebhookRepository) ListActiveByEvent(ctx context.Context, event string) ([]Webhook, error) {
	eventsJSON, _ := json.Marshal([]string{event})
	rows, err := r.pool.Query(ctx, `
		SELECT id, public_id, url, signing_secret, events, status, created_by, created_at, updated_at
		FROM webhooks
		WHERE status = 'active' AND events @> $1::jsonb
	`, eventsJSON)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Webhook
	for rows.Next() {
		w, err := scanWebhook(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *w)
	}
	return list, rows.Err()
}

func (r *WebhookRepository) Delete(ctx context.Context, publicID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM webhooks WHERE public_id = $1`, publicID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrWebhookNotFound
	}
	return nil
}
