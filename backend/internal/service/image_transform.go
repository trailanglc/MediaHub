package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/anhtuanlc/mediahub/internal/integration"
	"github.com/anhtuanlc/mediahub/internal/mediautil"
	"github.com/anhtuanlc/mediahub/internal/platform/rediscache"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/storage"
)

const maxTransformSourceBytes = 25 * 1024 * 1024

type ImageTransformService struct {
	store    storage.ObjectStorage
	cache    *rediscache.Store
	cacheTTL time.Duration
}

func NewImageTransformService(store storage.ObjectStorage, cache *rediscache.Store, cacheTTL time.Duration) *ImageTransformService {
	if cacheTTL <= 0 {
		cacheTTL = 24 * time.Hour
	}
	return &ImageTransformService{store: store, cache: cache, cacheTTL: cacheTTL}
}

type ImageTransformRequest struct {
	ObjectID string
	Variant  string
	Width    int
	Height   int
	Format   string
}

func (s *ImageTransformService) TransformableVariant(variant string) bool {
	return variant == integration.AssetVariantImage || variant == integration.AssetVariantThumbnail
}

func (s *ImageTransformService) Render(
	ctx context.Context,
	m *repository.MediaObject,
	req ImageTransformRequest,
) ([]byte, string, error) {
	if s == nil || s.store == nil {
		return nil, "", fmt.Errorf("transform unavailable")
	}
	if !req.HasTransform() {
		return nil, "", fmt.Errorf("no transform requested")
	}
	if !s.TransformableVariant(req.Variant) {
		return nil, "", fmt.Errorf("variant not transformable")
	}
	if m.Type != "image" && req.Variant == integration.AssetVariantImage {
		return nil, "", ErrMediaAccessDenied
	}

	sourceKey, _, err := StorageKeyForVariant(m, req.Variant)
	if err != nil {
		return nil, "", err
	}

	cacheKey := transformCacheKey(req, sourceKey)
	if s.cache != nil && s.cache.Enabled() {
		if raw, ok, err := s.cache.GetBytes(ctx, cacheKey); err == nil && ok {
			return raw, cacheContentType(req.Format), nil
		}
	}

	body, err := s.store.GetObject(ctx, sourceKey)
	if err != nil {
		return nil, "", err
	}
	defer body.Close()
	src, err := io.ReadAll(io.LimitReader(body, maxTransformSourceBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(src)) > maxTransformSourceBytes {
		return nil, "", fmt.Errorf("source too large for transform")
	}

	opts := mediautil.TransformOptions{
		Width:  mediautil.ClampTransformEdge(req.Width),
		Height: mediautil.ClampTransformEdge(req.Height),
		Format: req.Format,
	}
	out, ct, err := mediautil.TransformImage(src, opts)
	if err != nil {
		return nil, "", err
	}

	if s.cache != nil && s.cache.Enabled() {
		_ = s.cache.SetBytes(ctx, cacheKey, out, s.cacheTTL)
	}
	return out, ct, nil
}

func (r ImageTransformRequest) HasTransform() bool {
	return r.Width > 0 || r.Height > 0 || mediautil.NormalizeTransformFormat(r.Format) != ""
}

func transformCacheKey(req ImageTransformRequest, sourceKey string) string {
	sum := sha256.Sum256([]byte(sourceKey))
	return "cache:imgtx:" + req.ObjectID + ":" + req.Variant + ":" +
		strconv.Itoa(req.Width) + "x" + strconv.Itoa(req.Height) + ":" +
		mediautil.NormalizeTransformFormat(req.Format) + ":" + hex.EncodeToString(sum[:8])
}

func cacheContentType(format string) string {
	switch mediautil.NormalizeTransformFormat(format) {
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}
