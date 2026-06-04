package handler

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/anhtuanlc/mediahub/internal/authz"
	"github.com/anhtuanlc/mediahub/internal/middleware"
	"github.com/anhtuanlc/mediahub/internal/platform/rediscache"
	streamplat "github.com/anhtuanlc/mediahub/internal/platform/stream"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/anhtuanlc/mediahub/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	maxHLSPlaylistBytes = 2 << 20 // 2 MiB
	streamCookieToken   = "mh_stream"
	streamCookieExp     = "mh_stream_exp"
)

type StreamHandler struct {
	streamLoader     *rediscache.StreamLoader
	streamTok        *service.StreamTokenService
	store            storage.ObjectStorage
	apiKeys          *service.APIKeyService
	integration      *service.IntegrationService
	metrics          *streamplat.Metrics
	rateLimiter      *streamplat.RateLimiter
	playlistCache    *playlistBodyCache
	appURL           string
	cookieSecure     bool
	authMW           *middleware.AuthMiddleware
	authz            *authz.Service
	segmentURLWindow time.Duration
	internalRedirect string
}

// StreamHandlerOptions carries optional performance tuning for HLS delivery.
type StreamHandlerOptions struct {
	// SegmentURLWindow is the rolling validity window for shared, edge-cacheable signed
	// segment URLs. Zero falls back to one hour.
	SegmentURLWindow time.Duration
	// InternalRedirectPrefix, when non-empty, makes segment responses emit X-Accel-Redirect
	// to an internal nginx location instead of streaming bytes through the API.
	InternalRedirectPrefix string
}

func NewStreamHandler(
	streamLoader *rediscache.StreamLoader,
	streamTok *service.StreamTokenService,
	store storage.ObjectStorage,
	apiKeys *service.APIKeyService,
	integrationSvc *service.IntegrationService,
	metrics *streamplat.Metrics,
	rateLimiter *streamplat.RateLimiter,
	appURL string,
	appEnv string,
	authMW *middleware.AuthMiddleware,
	authzSvc *authz.Service,
	opts StreamHandlerOptions,
) *StreamHandler {
	secure := appEnv == "production" || appEnv == "staging"
	window := opts.SegmentURLWindow
	if window <= 0 {
		window = time.Hour
	}
	pc := newPlaylistBodyCache()
	h := &StreamHandler{
		streamLoader:     streamLoader,
		streamTok:        streamTok,
		store:            store,
		apiKeys:          apiKeys,
		integration:      integrationSvc,
		metrics:          metrics,
		rateLimiter:      rateLimiter,
		playlistCache:    pc,
		appURL:           strings.TrimSpace(appURL),
		cookieSecure:     secure,
		authMW:           authMW,
		authz:            authzSvc,
		segmentURLWindow: window,
		internalRedirect: strings.TrimRight(strings.TrimSpace(opts.InternalRedirectPrefix), "/"),
	}
	// Drop per-process playlist bodies when any replica invalidates a video (re-convert,
	// HLS delete, policy change), so a load-balanced fleet never serves a stale manifest.
	if streamLoader != nil && streamLoader.Store != nil {
		streamLoader.Store.Subscribe(context.Background(), rediscache.ChannelStreamInvalidate, func(videoID string) {
			pc.InvalidateVideo(videoID)
		})
	}
	return h
}

func isHLSPlaylistPath(filePath string) bool {
	return strings.HasSuffix(strings.ToLower(filePath), ".m3u8")
}

func (h *StreamHandler) Serve(c *gin.Context) {
	videoPID, err := uuid.Parse(c.Param("video_public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid video id"})
		return
	}
	videoID := videoPID.String()

	filePath := strings.TrimPrefix(c.Param("filepath"), "/")
	if filePath == "" {
		filePath = "master.m3u8"
	}
	filePath, pathOK := sanitizeStreamFilePath(filePath, videoID)
	if !pathOK {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid stream path"})
		return
	}

	isPlaylist := isHLSPlaylistPath(filePath)
	storageKey := storage.HLSPrefix(videoPID) + filePath

	// Fast path: segments carrying a valid shared signature are authorized without a Redis
	// lookup, cookie, or origin check. They get a stable (cookie-free) response so a CDN can
	// cache each segment once and fan it out to every viewer.
	if !isPlaylist {
		if exp, _ := strconv.ParseInt(c.Query("e"), 10, 64); exp > 0 {
			if h.streamTok.VerifySegment(videoID, c.Query("s"), exp) == nil {
				if h.rateLimiter != nil && !h.rateLimiter.AllowSegment(c.Request.Context(), c.ClientIP(), videoID) {
					c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited", "message": "too many stream requests"})
					return
				}
				h.serveSegment(c, videoID, storageKey, filePath, true)
				return
			}
		}
	}

	if h.streamLoader == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	sctx, err := h.streamLoader.Get(c.Request.Context(), videoPID)
	if err != nil || sctx == nil || sctx.HLSStatus != "ready" {
		c.JSON(http.StatusForbidden, gin.H{"error": "stream_unavailable", "message": "HLS not ready"})
		return
	}

	if !h.streamOriginAllowed(c, sctx.AllowedDomains, sctx.GlobalAllowedDomains, sctx.ObjectID) {
		plainKey := strings.TrimSpace(c.GetHeader("X-API-Key"))
		if plainKey == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "domain not allowed"})
			return
		}
		key, err := h.apiKeys.VerifyAPIKey(c.Request.Context(), plainKey)
		if err != nil || !h.apiKeys.HasScope(key, "stream") {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "invalid api key"})
			return
		}
		if !h.apiKeys.CheckIPRestriction(c.ClientIP(), key) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "api key ip restriction"})
			return
		}
		if h.integration != nil {
			if err := h.integration.ObjectInNamespace(c.Request.Context(), key, sctx.ObjectID); err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "api key namespace restriction"})
				return
			}
		}
	}

	token, expUnix, ok := h.resolveSession(c, videoID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "invalid or expired stream session"})
		return
	}

	if isPlaylist && h.rateLimiter != nil {
		allowed, rlErr := h.rateLimiter.Allow(c.Request.Context(), c.ClientIP(), videoID)
		if rlErr != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable", "message": "stream rate limit unavailable"})
			return
		}
		if !allowed {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited", "message": "too many stream requests"})
			return
		}
	}

	if isPlaylist {
		h.servePlaylist(c, videoID, filePath, storageKey, token, expUnix)
		return
	}
	if h.rateLimiter != nil && !h.rateLimiter.AllowSegment(c.Request.Context(), c.ClientIP(), videoID) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited", "message": "too many stream requests"})
		return
	}
	h.serveSegment(c, videoID, storageKey, filePath, false)
}

func (h *StreamHandler) resolveSession(c *gin.Context, videoID string) (token string, expUnix int64, ok bool) {
	if tok, err := c.Cookie(streamCookieToken); err == nil && tok != "" {
		if expStr, err := c.Cookie(streamCookieExp); err == nil {
			exp, _ := strconv.ParseInt(expStr, 10, 64)
			if h.streamTok.Verify(videoID, tok, exp) == nil {
				return tok, exp, true
			}
		}
	}

	qTok := strings.TrimSpace(c.Query("token"))
	qExp, _ := strconv.ParseInt(c.Query("exp"), 10, 64)
	if qTok != "" && h.streamTok.Verify(videoID, qTok, qExp) == nil {
		return qTok, qExp, true
	}
	return "", 0, false
}

func streamCookiePath(videoID string) string {
	return "/stream/" + videoID
}

func (h *StreamHandler) setSessionCookies(c *gin.Context, videoID, token string, expUnix int64) {
	maxAge := int(expUnix - time.Now().Unix())
	if maxAge < 0 {
		maxAge = 0
	}
	path := streamCookiePath(videoID)
	sameSite := http.SameSiteLaxMode
	secure := false
	if h.cookieSecure {
		sameSite = http.SameSiteNoneMode
		secure = true
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     streamCookieToken,
		Value:    token,
		Path:     path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     streamCookieExp,
		Value:    strconv.FormatInt(expUnix, 10),
		Path:     path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
}

func (h *StreamHandler) servePlaylist(c *gin.Context, videoID, filePath, storageKey, token string, expUnix int64) {
	body, hit := h.playlistCache.Get(videoID, filePath)
	if !hit {
		raw, err := h.store.GetObject(c.Request.Context(), storageKey)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "playlist not found"})
			return
		}
		defer raw.Close()
		limited := io.LimitReader(raw, maxHLSPlaylistBytes+1)
		data, err := io.ReadAll(limited)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
			return
		}
		if int64(len(data)) > maxHLSPlaylistBytes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "playlist too large"})
			return
		}
		// Cache the rewritten body with relative, unsigned URLs. Per-window signatures are
		// applied per response below so the cached entry stays stable across windows.
		body = []byte(normalizePlaylistURLs(string(data), videoID, filePath))
		h.playlistCache.Set(videoID, filePath, body)
	}

	out := h.signPlaylistBody(videoID, body)
	etag := weakETag(out)

	h.setSessionCookies(c, videoID, token, expUnix)
	setStreamCORS(c, h.appURL)
	c.Header("Cache-Control", "private, max-age=60")
	c.Header("ETag", etag)
	if etagMatches(c.GetHeader("If-None-Match"), etag) {
		h.recordAccessAsync(videoID, 0)
		c.Status(http.StatusNotModified)
		return
	}
	h.recordAccessAsync(videoID, int64(len(out)))
	c.Data(http.StatusOK, "application/vnd.apple.mpegurl", out)
}

// signPlaylistBody appends a shared (per-video, per-window) signature to every segment line.
// Variant playlist references (.m3u8) are left untouched: they stay session-authorized via Go.
func (h *StreamHandler) signPlaylistBody(videoID string, body []byte) []byte {
	if h.streamTok == nil {
		return body
	}
	exp := service.SegmentExpiry(time.Now(), h.segmentURLWindow)
	sig := h.streamTok.SignSegment(videoID, exp)
	query := "e=" + strconv.FormatInt(exp, 10) + "&s=" + sig

	lines := strings.Split(string(body), "\n")
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(strings.Split(trimmed, "?")[0]), ".m3u8") {
			continue
		}
		sep := "?"
		if strings.Contains(line, "?") {
			sep = "&"
		}
		lines[i] = line + sep + query
		changed = true
	}
	if !changed {
		return body
	}
	return []byte(strings.Join(lines, "\n"))
}

func (h *StreamHandler) serveSegment(c *gin.Context, videoID, storageKey, filePath string, cacheable bool) {
	if cacheable {
		setSegmentPublicCORS(c)
	} else {
		setStreamCORS(c, h.appURL)
	}
	ct := streamContentType(filePath)

	// Offload byte delivery to nginx (which streams directly from object storage) so the API
	// never touches segment bytes. The edge still caches the response by its shared signed URL.
	if h.internalRedirect != "" {
		hdr := c.Writer.Header()
		hdr.Set("Cache-Control", "public, max-age=31536000, immutable")
		hdr.Set("Accept-Ranges", "bytes")
		hdr.Set("Content-Type", ct)
		hdr.Set("X-Accel-Redirect", h.internalRedirect+"/"+storageKey)
		h.recordAccessAsync(videoID, 0)
		c.Status(http.StatusOK)
		return
	}

	ctx := c.Request.Context()
	rangeHeader := c.GetHeader("Range")
	body, info, err := h.store.GetRange(ctx, storageKey, rangeHeader)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "segment not found"})
		return
	}
	defer body.Close()

	if info.ContentType != "" {
		ct = info.ContentType
	}
	setImmutableCacheHeaders(c.Writer.Header(), info.ETag)

	if info.ETag != "" && etagMatches(c.GetHeader("If-None-Match"), "\""+info.ETag+"\"") {
		c.Status(http.StatusNotModified)
		return
	}

	c.Header("Content-Type", ct)
	c.Header("Content-Length", strconv.FormatInt(info.Size, 10))
	if info.ContentRange != "" {
		c.Header("Content-Range", info.ContentRange)
		c.Status(http.StatusPartialContent)
	} else {
		c.Status(http.StatusOK)
	}
	_, _ = io.Copy(c.Writer, body)
	h.recordAccessAsync(videoID, info.Size)
}

func (h *StreamHandler) recordAccessAsync(videoID string, bytes int64) {
	if h.metrics == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = h.metrics.RecordAccess(ctx, videoID, bytes)
	}()
}

func requestOrigin(c *gin.Context) string {
	origin := c.GetHeader("Origin")
	if origin == "" {
		origin = c.GetHeader("Referer")
	}
	return origin
}

func (h *StreamHandler) streamOriginAllowed(c *gin.Context, allowed, global []string, objectID int64) bool {
	origin := requestOrigin(c)
	if service.MatchDomainAllowlist(origin, allowed, global) {
		return true
	}
	if h.appURL != "" && service.MatchDomainAllowlist(origin, []string{h.appURL}, nil) {
		return true
	}
	if h.authMW != nil && h.authz != nil {
		if u, ok := h.authMW.TryAuth(c); ok && h.userCanStream(c, u, objectID) {
			return true
		}
	}
	return false
}

func (h *StreamHandler) userCanStream(c *gin.Context, u middleware.AuthUser, objectID int64) bool {
	if authz.IsOwnerRole(u.Role) {
		return true
	}
	ok, err := h.authz.HasPermission(c.Request.Context(), u.ID, u.Role, objectID, authz.ActionStream)
	return err == nil && ok
}

// normalizePlaylistURLs rewrites playlist lines to relative paths (no per-segment tokens).
func normalizePlaylistURLs(content, videoID, playlistPath string) string {
	playlistDir := path.Dir(strings.ReplaceAll(playlistPath, "\\", "/"))
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if rel, ok := normalizeStreamResource(trimmed, videoID, playlistDir); ok {
			lines[i] = rel
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func setStreamCORS(c *gin.Context, appURL string) {
	origin := c.GetHeader("Origin")
	allow := strings.TrimSpace(appURL)
	if origin != "" && (allow == "" || service.MatchDomainAllowlist(origin, []string{allow}, nil)) {
		c.Header("Access-Control-Allow-Origin", origin)
	} else if allow != "" {
		c.Header("Access-Control-Allow-Origin", allow)
	}
	c.Header("Access-Control-Allow-Credentials", "true")
	c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type, Content-Range")
}

// setSegmentPublicCORS emits cookie-free CORS for signed segments. The body is identical for
// every viewer (authorized purely by the shared URL signature), so the response is cacheable at
// the edge. We echo the request Origin (with credentials) when present so credentialed players
// such as Safari native HLS keep working; only the ACAO header varies by Origin (Vary: Origin),
// not the body. When there is no Origin (e.g. native media fetches), we allow any origin.
func setSegmentPublicCORS(c *gin.Context) {
	origin := c.GetHeader("Origin")
	if origin != "" {
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Vary", "Origin")
	} else {
		c.Header("Access-Control-Allow-Origin", "*")
	}
	c.Header("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Range")
	c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type, Content-Range, Accept-Ranges")
}

func sanitizeStreamFilePath(filePath, videoID string) (string, bool) {
	filePath = strings.TrimPrefix(strings.TrimSpace(filePath), "/")
	marker := "stream/" + videoID + "/"
	var raw string
	if idx := strings.Index(filePath, marker); idx >= 0 {
		raw = strings.Split(filePath[idx+len(marker):], "?")[0]
	} else if idx := strings.Index(filePath, "://"); idx >= 0 {
		rest := filePath[idx+3:]
		if slash := strings.Index(rest, "/"); slash >= 0 {
			rest = rest[slash+1:]
			if i2 := strings.Index(rest, marker); i2 >= 0 {
				raw = strings.Split(rest[i2+len(marker):], "?")[0]
			}
		}
	} else {
		raw = strings.Split(filePath, "?")[0]
	}
	return validateStreamRelativePath(raw)
}

func validateStreamRelativePath(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	normalized := strings.ReplaceAll(raw, "\\", "/")
	if strings.Contains(normalized, "..") {
		return "", false
	}
	cleaned := path.Clean("/" + normalized)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" || cleaned == "." {
		return "", false
	}
	if strings.Contains(cleaned, "..") {
		return "", false
	}
	return cleaned, true
}

func normalizeStreamResource(line, videoID, playlistDir string) (string, bool) {
	storageRel, ok := toHLSRootRelative(line, videoID)
	if !ok {
		return "", false
	}
	return toPlaylistRelative(storageRel, playlistDir), true
}

// toHLSRootRelative maps a playlist line to a path relative to the video HLS root (e.g. 720p/segment_00001.ts).
func toHLSRootRelative(line, videoID string) (string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false
	}
	line = strings.Split(line, "?")[0]
	line = strings.TrimPrefix(line, "/")

	marker := "stream/" + videoID + "/"
	if idx := strings.Index(line, marker); idx >= 0 {
		rest := strings.TrimPrefix(line[idx:], marker)
		rest = strings.TrimPrefix(rest, "/")
		if rest != "" && !strings.Contains(rest, "://") {
			return rest, true
		}
	}

	if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
		u, err := url.Parse(line)
		if err != nil {
			return "", false
		}
		p := strings.TrimPrefix(u.Path, "/")
		if strings.HasPrefix(p, marker) {
			return strings.TrimPrefix(p, marker), true
		}
		return "", false
	}

	if strings.Contains(line, "://") {
		return "", false
	}
	return line, true
}

// toPlaylistRelative rewrites HLS-root paths for URL resolution against the served playlist file.
// Variant media playlists live under e.g. 720p/index.m3u8 — segments must stay segment_00001.ts,
// not 720p/segment_00001.ts, or clients request .../720p/720p/segment_00001.ts.
func toPlaylistRelative(storageRel, playlistDir string) string {
	playlistDir = strings.ReplaceAll(playlistDir, "\\", "/")
	if playlistDir == "." || playlistDir == "" {
		return storageRel
	}
	prefix := playlistDir + "/"
	if strings.HasPrefix(storageRel, prefix) {
		return strings.TrimPrefix(storageRel, prefix)
	}
	return storageRel
}

func StreamForbidden(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{
		"error":   "stream_unavailable",
		"message": "HLS streaming is not enabled until video conversion is complete",
	})
}
