package service

import (
	"strings"
	"time"

	"github.com/anhtuanlc/mediahub/internal/integration"
	"github.com/anhtuanlc/mediahub/internal/repository"
)

// DeliveryService builds cache-friendly signed URLs for external consumption.
type DeliveryService struct {
	tokens      *StreamTokenService
	baseURL     string
	assetWindow time.Duration
}

func NewDeliveryService(tokens *StreamTokenService, baseURL string, assetWindow time.Duration) *DeliveryService {
	if assetWindow <= 0 {
		assetWindow = 24 * time.Hour
	}
	return &DeliveryService{
		tokens:      tokens,
		baseURL:     strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		assetWindow: assetWindow,
	}
}

// MediaDeliveryURLs holds signed URLs for embedding and hotlinking.
type MediaDeliveryURLs struct {
	ThumbnailURL *string `json:"thumbnail_url,omitempty"`
	ImageURL     *string `json:"image_url,omitempty"`
	FileURL      *string `json:"file_url,omitempty"`
	HLSMasterURL *string `json:"hls_master_url,omitempty"`
	EmbedURL     *string `json:"embed_url,omitempty"`
	EmbedHTML    *string `json:"embed_html,omitempty"`
	CacheUntil   int64   `json:"cache_until"`
}

func (d *DeliveryService) cacheUntil() int64 {
	return SegmentExpiry(time.Now(), d.assetWindow)
}

func (d *DeliveryService) BuildForObject(m *repository.MediaObject, includeHLS bool, hlsMasterURL string, allowSourceDownload bool) MediaDeliveryURLs {
	if d == nil || d.tokens == nil || m == nil {
		return MediaDeliveryURLs{}
	}
	pid := m.PublicID.String()
	out := MediaDeliveryURLs{CacheUntil: d.cacheUntil()}
	if m.ThumbnailKey != nil && *m.ThumbnailKey != "" {
		u := d.tokens.BuildAssetURL(d.baseURL, pid, integration.AssetVariantThumbnail, d.assetWindow)
		out.ThumbnailURL = &u
	}
	switch m.Type {
	case "image":
		u := d.tokens.BuildAssetURL(d.baseURL, pid, integration.AssetVariantImage, d.assetWindow)
		out.ImageURL = &u
		if out.ThumbnailURL == nil {
			out.ThumbnailURL = &u
		}
	case "file":
		if m.StorageKey != nil && *m.StorageKey != "" {
			u := d.tokens.BuildAssetURL(d.baseURL, pid, integration.AssetVariantFile, d.assetWindow)
			out.FileURL = &u
		}
	case "video":
		if includeHLS && hlsMasterURL != "" {
			out.HLSMasterURL = &hlsMasterURL
		}
		embedURL := d.tokens.BuildEmbedURL(d.baseURL, pid, d.assetWindow)
		out.EmbedURL = &embedURL
		html := d.BuildEmbedHTML(pid)
		out.EmbedHTML = &html
		if allowSourceDownload && m.StorageKey != nil && *m.StorageKey != "" {
			u := d.tokens.BuildAssetURL(d.baseURL, pid, integration.AssetVariantFile, d.assetWindow)
			out.FileURL = &u
		}
	}
	return out
}

func (d *DeliveryService) BuildEmbedHTML(videoPublicID string) string {
	embedURL := d.tokens.BuildEmbedURL(d.baseURL, videoPublicID, d.assetWindow)
	return `<iframe src="` + embedURL + `" allow="autoplay; encrypted-media; fullscreen" allowfullscreen style="width:100%;aspect-ratio:16/9;border:0"></iframe>`
}

func (d *DeliveryService) VerifyAsset(objectID string, variant string, expUnix int64, sig string) error {
	return d.tokens.VerifyAsset(objectID, variant, expUnix, sig)
}

func (d *DeliveryService) AssetWindow() time.Duration {
	return d.assetWindow
}

func (d *DeliveryService) BaseURL() string {
	return d.baseURL
}
