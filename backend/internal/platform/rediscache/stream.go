package rediscache

import (
	"context"
	"errors"

	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/google/uuid"
)

// StreamContext is cached metadata for HLS stream requests.
type StreamContext struct {
	HLSStatus            string   `json:"hls_status"`
	ObjectID             int64    `json:"object_id"`
	AllowedDomains       []string `json:"allowed_domains"`
	TokenTTLSeconds      int      `json:"token_ttl_seconds"`
	GlobalAllowedDomains []string `json:"global_allowed_domains"`
}

// GlobalDomainsProvider returns streaming global allowlist (e.g. from settings).
type GlobalDomainsProvider func(ctx context.Context) ([]string, error)

// StreamLoader loads and caches stream context per video public id.
type StreamLoader struct {
	Store         *Store
	Videos        *repository.VideoRepository
	GlobalDomains GlobalDomainsProvider
}

func (l *StreamLoader) Enabled() bool {
	return l != nil && l.Store != nil && l.Store.Enabled() && l.Videos != nil
}

func (l *StreamLoader) Get(ctx context.Context, videoPublicID uuid.UUID) (*StreamContext, error) {
	if l == nil || l.Videos == nil {
		return nil, errors.New("stream loader not configured")
	}
	key := KeyStreamVideo(videoPublicID.String())
	if !l.Enabled() {
		return l.load(ctx, videoPublicID, nil)
	}
	var out StreamContext
	_, err := l.Store.GetOrLoadJSON(ctx, key, TTLStreamVideo, &out, func(ctx context.Context) (any, error) {
		return l.load(ctx, videoPublicID, nil)
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (l *StreamLoader) load(ctx context.Context, videoPublicID uuid.UUID, global []string) (*StreamContext, error) {
	row, err := l.Videos.GetByObjectPublicID(ctx, videoPublicID)
	if err != nil {
		return nil, err
	}
	if global == nil && l.GlobalDomains != nil {
		global, _ = l.GlobalDomains(ctx)
	}
	allowed := []string{}
	tokenTTL := 3600
	pol, err := l.Videos.GetOrCreateStreamPolicy(ctx, row.Asset.ID, 3600)
	if err == nil && pol != nil {
		allowed = append([]string(nil), pol.AllowedDomains...)
		if pol.TokenTTLSeconds > 0 {
			tokenTTL = pol.TokenTTLSeconds
		}
	}
	return &StreamContext{
		HLSStatus:            row.Asset.HLSStatus,
		ObjectID:             row.Media.ID,
		AllowedDomains:       allowed,
		TokenTTLSeconds:      tokenTTL,
		GlobalAllowedDomains: global,
	}, nil
}

func InvalidateStreamVideo(ctx context.Context, store *Store, videoPublicID uuid.UUID) {
	if store == nil || !store.Enabled() {
		return
	}
	_ = store.Delete(ctx, KeyStreamVideo(videoPublicID.String()))
	// Notify all API replicas to drop per-process playlist bodies for this video.
	store.Publish(ctx, ChannelStreamInvalidate, videoPublicID.String())
}

func InvalidateAllStreamVideos(ctx context.Context, store *Store) {
	if store == nil || !store.Enabled() {
		return
	}
	_ = store.DeleteByPrefix(ctx, PrefixStreamVideo)
}
