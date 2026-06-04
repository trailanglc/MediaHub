package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/anhtuanlc/mediahub/internal/repository"
	"go.uber.org/zap"
)

const dispatchTimeout = 8 * time.Second

type Dispatcher struct {
	repo   *repository.WebhookRepository
	client *http.Client
	log    *zap.Logger
}

func NewDispatcher(repo *repository.WebhookRepository, log *zap.Logger) *Dispatcher {
	return &Dispatcher{
		repo: repo,
		client: &http.Client{
			Timeout: dispatchTimeout,
		},
		log: log,
	}
}

// Payload is the JSON body sent to subscriber endpoints.
type Payload struct {
	Event     string         `json:"event"`
	Timestamp int64          `json:"timestamp"`
	Data      map[string]any `json:"data"`
}

func (d *Dispatcher) Emit(ctx context.Context, event string, data map[string]any) {
	if d == nil || d.repo == nil {
		return
	}
	hooks, err := d.repo.ListActiveByEvent(ctx, event)
	if err != nil || len(hooks) == 0 {
		return
	}
	body, err := json.Marshal(Payload{
		Event:     event,
		Timestamp: time.Now().UTC().Unix(),
		Data:      data,
	})
	if err != nil {
		return
	}
	for i := range hooks {
		hook := hooks[i]
		go d.deliver(hook.URL, hook.SigningSecret, body)
	}
}

func (d *Dispatcher) deliver(url, secret string, body []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), dispatchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	ts := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	sig := signBody(secret, ts, body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MediaHub-Timestamp", ts)
	req.Header.Set("X-MediaHub-Signature", "sha256="+sig)

	resp, err := d.client.Do(req)
	if err != nil {
		if d.log != nil {
			d.log.Warn("webhook delivery failed", zap.String("url", url), zap.Error(err))
		}
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 && d.log != nil {
		d.log.Warn("webhook non-2xx", zap.String("url", url), zap.Int("status", resp.StatusCode))
	}
}

func signBody(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
