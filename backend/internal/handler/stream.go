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
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/anhtuanlc/mediahub/internal/storage"
	streamplat "github.com/anhtuanlc/mediahub/internal/platform/stream"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	maxHLSPlaylistBytes = 2 << 20 // 2 MiB
	streamCookieToken   = "mh_stream"
	streamCookieExp     = "mh_stream_exp"
)

type StreamHandler struct {
	streamLoader  *rediscache.StreamLoader
	streamTok     *service.StreamTokenService
	store         storage.ObjectStorage
	apiKeys       *service.APIKeyService
	metrics       *streamplat.Metrics
	rateLimiter   *streamplat.RateLimiter
	playlistCache *playlistBodyCache
	appURL        string
	cookieSecure  bool
	authMW        *middleware.AuthMiddleware
	authz         *authz.Service
}

func NewStreamHandler(
	streamLoader *rediscache.StreamLoader,
	streamTok *service.StreamTokenService,
	store storage.ObjectStorage,
	apiKeys *service.APIKeyService,
	metrics *streamplat.Metrics,
	rateLimiter *streamplat.RateLimiter,
	appURL string,
	appEnv string,
	authMW *middleware.AuthMiddleware,
	authzSvc *authz.Service,
) *StreamHandler {
	secure := appEnv == "production" || appEnv == "staging"
	return &StreamHandler{
		streamLoader:  streamLoader,
		streamTok:     streamTok,
		store:         store,
		apiKeys:       apiKeys,
		metrics:       metrics,
		rateLimiter:   rateLimiter,
		playlistCache: newPlaylistBodyCache(),
		appURL:        strings.TrimSpace(appURL),
		cookieSecure:  secure,
		authMW:        authMW,
		authz:         authzSvc,
	}
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
		origin := requestOrigin(c)
		if !h.apiKeys.MatchIP(c.ClientIP(), key.AllowedIPs) || !h.apiKeys.MatchDomain(key, origin) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "api key restrictions"})
			return
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

	storageKey := storage.HLSPrefix(videoPID) + filePath
	if isPlaylist {
		h.servePlaylist(c, videoID, filePath, storageKey, token, expUnix)
		return
	}
	h.serveSegment(c, storageKey, filePath)
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
	if body, hit := h.playlistCache.Get(videoID, filePath); hit {
		h.setSessionCookies(c, videoID, token, expUnix)
		setStreamCORS(c, h.appURL)
		c.Header("Cache-Control", "private, max-age=60")
		h.recordAccessAsync(videoID, int64(len(body)))
		c.Data(http.StatusOK, "application/vnd.apple.mpegurl", body)
		return
	}

	body, err := h.store.GetObject(c.Request.Context(), storageKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "playlist not found"})
		return
	}
	defer body.Close()
	limited := io.LimitReader(body, maxHLSPlaylistBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	if int64(len(data)) > maxHLSPlaylistBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "playlist too large"})
		return
	}

	rewritten := normalizePlaylistURLs(string(data), videoID, filePath)
	out := []byte(rewritten)
	h.playlistCache.Set(videoID, filePath, out)

	h.setSessionCookies(c, videoID, token, expUnix)
	setStreamCORS(c, h.appURL)
	c.Header("Cache-Control", "private, max-age=60")
	h.recordAccessAsync(videoID, int64(len(out)))
	c.Data(http.StatusOK, "application/vnd.apple.mpegurl", out)
}

func (h *StreamHandler) serveSegment(c *gin.Context, storageKey, filePath string) {
	setStreamCORS(c, h.appURL)
	ctx := c.Request.Context()
	info, err := h.store.StatObject(ctx, storageKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "segment not found"})
		return
	}

	ct := info.ContentType
	if ct == "" {
		ct = streamContentType(filePath)
	}
	setImmutableCacheHeaders(c.Writer.Header(), info.ETag)

	rangeHeader := c.GetHeader("Range")
	if rangeHeader == "" {
		body, _, err := h.store.GetObjectRange(ctx, storageKey, 0, -1)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "segment not found"})
			return
		}
		defer body.Close()
		c.Header("Content-Type", ct)
		c.Header("Content-Length", strconv.FormatInt(info.Size, 10))
		c.Status(http.StatusOK)
		_, _ = io.Copy(c.Writer, body)
		return
	}

	start, end, ok := parseByteRange(rangeHeader, info.Size)
	if !ok {
		c.Header("Content-Range", "bytes */"+strconv.FormatInt(info.Size, 10))
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	length := end - start + 1
	body, _, err := h.store.GetObjectRange(ctx, storageKey, start, length)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "segment not found"})
		return
	}
	defer body.Close()
	c.Header("Content-Type", ct)
	c.Header("Content-Length", strconv.FormatInt(length, 10))
	c.Header("Content-Range", "bytes "+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10)+"/"+strconv.FormatInt(info.Size, 10))
	c.Status(http.StatusPartialContent)
	_, _ = io.Copy(c.Writer, body)
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
