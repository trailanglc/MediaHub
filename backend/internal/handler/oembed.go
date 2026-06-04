package handler

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type OEmbedHandler struct {
	Videos       *repository.VideoRepository
	Objects      *repository.MediaObjectRepository
	Delivery     *service.DeliveryService
	ProviderName string
	ProviderURL  string
	AppURL       string
	APIBaseURL   string
}

func NewOEmbedHandler(
	videos *repository.VideoRepository,
	objects *repository.MediaObjectRepository,
	delivery *service.DeliveryService,
	providerURL, appURL, apiBaseURL string,
) *OEmbedHandler {
	return &OEmbedHandler{
		Videos:       videos,
		Objects:      objects,
		Delivery:     delivery,
		ProviderName: "MediaHub",
		ProviderURL:  strings.TrimRight(providerURL, "/"),
		AppURL:       strings.TrimRight(appURL, "/"),
		APIBaseURL:   strings.TrimRight(apiBaseURL, "/"),
	}
}

func (h *OEmbedHandler) Serve(c *gin.Context) {
	rawURL := strings.TrimSpace(c.Query("url"))
	if rawURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "url query required"})
		return
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid url"})
		return
	}

	videoPID, ok := h.parseVideoPublicID(u)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "unsupported oembed url"})
		return
	}

	ctx := c.Request.Context()
	row, err := h.Videos.GetByObjectPublicID(ctx, videoPID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "video not found"})
		return
	}

	title := row.Media.Name
	var thumbURL *string
	if h.Delivery != nil {
		urls := h.Delivery.BuildForObject(&row.Media, false, "", false)
		thumbURL = urls.ThumbnailURL
	}

	var html string
	if h.Delivery != nil {
		html = h.Delivery.BuildEmbedHTML(videoPID.String())
	}

	maxW, _ := parsePositiveInt(c.Query("maxwidth"))
	maxH, _ := parsePositiveInt(c.Query("maxheight"))
	width := 1280
	height := 720
	if row.Asset.Width != nil && row.Asset.Height != nil && *row.Asset.Width > 0 && *row.Asset.Height > 0 {
		width = *row.Asset.Width
		height = *row.Asset.Height
	}
	if maxW > 0 && maxW < width {
		width = maxW
	}
	if maxH > 0 && maxH < height {
		height = maxH
	}

	resp := gin.H{
		"version":       "1.0",
		"type":          "video",
		"provider_name": h.ProviderName,
		"provider_url":  h.ProviderURL,
		"title":         title,
		"width":         width,
		"height":        height,
	}
	if html != "" {
		resp["html"] = html
	}
	if thumbURL != nil {
		resp["thumbnail_url"] = *thumbURL
	}

	c.Header("Cache-Control", "public, max-age=300")
	c.JSON(http.StatusOK, resp)
}

func (h *OEmbedHandler) parseVideoPublicID(u *url.URL) (uuid.UUID, bool) {
	path := strings.Trim(u.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return uuid.Nil, false
	}
	var idStr string
	switch parts[len(parts)-2] {
	case "embed", "videos":
		idStr = parts[len(parts)-1]
	default:
		return uuid.Nil, false
	}
	idStr = strings.Split(idStr, "?")[0]
	pid, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, false
	}
	if !h.allowedHost(u.Host) {
		return uuid.Nil, false
	}
	return pid, true
}

func (h *OEmbedHandler) allowedHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	check := func(base string) bool {
		if base == "" {
			return false
		}
		pu, err := url.Parse(base)
		if err != nil {
			return false
		}
		return strings.EqualFold(strings.TrimSpace(pu.Host), host)
	}
	return check(h.ProviderURL) || check(h.AppURL) || check(h.APIBaseURL)
}

func parsePositiveInt(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	n := 0
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		n = n*10 + int(ch-'0')
	}
	if n <= 0 {
		return 0, false
	}
	return n, true
}
