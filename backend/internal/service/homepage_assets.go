package service

import (
	"context"
	"strings"
	"time"

	"github.com/anhtuanlc/mediahub/internal/integration"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/google/uuid"
)

const homepageAssetURLTTL = 365 * 24 * time.Hour

// HomepageAssetResolver turns stored media object IDs into signed delivery URLs.
type HomepageAssetResolver struct {
	objects  *repository.MediaObjectRepository
	delivery *DeliveryService
}

func NewHomepageAssetResolver(objects *repository.MediaObjectRepository, delivery *DeliveryService) *HomepageAssetResolver {
	return &HomepageAssetResolver{objects: objects, delivery: delivery}
}

func (r *HomepageAssetResolver) Resolve(ctx context.Context, hp SettingsHomepage) SettingsHomepage {
	if r == nil || r.delivery == nil || r.delivery.tokens == nil {
		return hp
	}
	if id := strings.TrimSpace(hp.FaviconObjectID); id != "" {
		hp.FaviconURL = r.assetURL(ctx, id, integration.AssetVariantImage, integration.AssetVariantFile)
	}
	if id := strings.TrimSpace(hp.OGImageObjectID); id != "" {
		hp.OGImageURL = r.assetURL(ctx, id, integration.AssetVariantImage)
	}
	if id := strings.TrimSpace(hp.HeroBackgroundObjectID); id != "" {
		hp.HeroBackgroundURL = r.assetURL(ctx, id, integration.AssetVariantImage)
	}
	return hp
}

func (r *HomepageAssetResolver) assetURL(ctx context.Context, publicID string, variants ...string) string {
	pid, err := uuid.Parse(publicID)
	if err != nil {
		return ""
	}
	m, err := r.objects.GetByPublicID(ctx, pid)
	if err != nil {
		return ""
	}
	switch m.Type {
	case "image", "file":
	default:
		return ""
	}
	for _, variant := range variants {
		u := r.delivery.tokens.BuildAssetURL(r.delivery.baseURL, pid.String(), variant, homepageAssetURLTTL)
		if u != "" {
			return u
		}
	}
	return ""
}
