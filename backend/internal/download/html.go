package download

import (
	"context"
	"html"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const maxHTMLCandidates = 15

var (
	// Match media URLs and stop at the extension (optional query only).
	reM3U8 = regexp.MustCompile(`(?i)https?://[^\s"'<>\\]+\.m3u8(?:\?[^\s"'<>\\]*)?`)
	reMP4  = regexp.MustCompile(`(?i)https?://[^\s"'<>\\]+\.mp4(?:\?[^\s"'<>\\]*)?`)
	reWebM = regexp.MustCompile(`(?i)https?://[^\s"'<>\\]+\.webm(?:\?[^\s"'<>\\]*)?`)

	reOGVideo  = regexp.MustCompile(`(?i)<meta[^>]+property=["']og:video(?::url)?["'][^>]+content=["']([^"']+)["']`)
	reOGVideo2 = regexp.MustCompile(`(?i)<meta[^>]+content=["']([^"']+)["'][^>]+property=["']og:video(?::url)?["']`)
	reOGImage  = regexp.MustCompile(`(?i)<meta[^>]+property=["']og:image["'][^>]+content=["']([^"']+)["']`)
	reOGImage2 = regexp.MustCompile(`(?i)<meta[^>]+content=["']([^"']+)["'][^>]+property=["']og:image["']`)
	reOGTitle  = regexp.MustCompile(`(?i)<meta[^>]+property=["']og:title["'][^>]+content=["']([^"']+)["']`)
	reTitle    = regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
	reSource   = regexp.MustCompile(`(?i)<source[^>]+src=["']([^"']+)["']`)
	reVideo    = regexp.MustCompile(`(?i)<video[^>]+src=["']([^"']+)["']`)
)

// Path fragments that usually mean thumbnails / related teasers, not the main media.
var noisePathFragments = []string{
	"/heat-preview/",
	"/heatmap",
	"/sprite",
	"/sprites/",
	"/poster/",
	"/posters/",
	"/thumbnail",
	"/thumbnails/",
	"/thumbs/",
	"/preview/",
	"/previews/",
	"/teaser/",
	"/trailers/preview",
	"/static/preview",
	"fh_heatmap_preview",
	"heatmap_preview",
}

// AnalyzeHTML fetches a page and extracts media candidates.
func AnalyzeHTML(ctx context.Context, rawURL string, timeout time.Duration) ([]Candidate, string, error) {
	u, err := ValidateURL(rawURL)
	if err != nil {
		return nil, "", err
	}
	if err := ResolveAndCheck(ctx, u); err != nil {
		return nil, "", err
	}

	client := SafeHTTPClient(timeout)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MiB
	if err != nil {
		return nil, "", err
	}
	page := string(body)
	base := u
	if resp.Request != nil && resp.Request.URL != nil {
		base = resp.Request.URL
	}

	title := firstMatch(reOGTitle, page)
	if title == "" {
		title = strings.TrimSpace(firstMatch(reTitle, page))
	}
	if title == "" {
		title = "Video"
	}
	thumb := firstMatch(reOGImage, page)
	if thumb == "" {
		thumb = firstMatch(reOGImage2, page)
	}
	thumb = absURL(base, thumb)

	seen := map[string]bool{}
	var cands []Candidate
	add := func(raw string, prefer bool) {
		if len(cands) >= maxHTMLCandidates {
			return
		}
		abs := absURL(base, cleanURL(raw))
		if abs == "" || seen[abs] {
			return
		}
		if !isPlausibleMediaURL(abs) {
			return
		}
		// Structured tags (og/source/video) are trusted more; still drop clear noise.
		if isNoiseMediaURL(abs) && !prefer {
			return
		}
		if isNoiseMediaURL(abs) && prefer {
			// og:video pointing at a heatmap preview is still noise.
			if isStrongNoiseMediaURL(abs) {
				return
			}
		}
		if _, err := ValidateURL(abs); err != nil {
			return
		}
		seen[abs] = true
		lower := strings.ToLower(abs)
		cands = append(cands, Candidate{
			ID:        strconv.Itoa(len(cands) + 1),
			Title:     title,
			URL:       abs,
			Ext:       guessExt(abs),
			Thumbnail: thumb,
			IsHLS:     strings.Contains(lower, ".m3u8"),
		})
	}

	// Prefer structured HTML first (main player sources).
	for _, m := range reOGVideo.FindAllStringSubmatch(page, -1) {
		if len(m) > 1 {
			add(m[1], true)
		}
	}
	for _, m := range reOGVideo2.FindAllStringSubmatch(page, -1) {
		if len(m) > 1 {
			add(m[1], true)
		}
	}
	for _, m := range reSource.FindAllStringSubmatch(page, -1) {
		if len(m) > 1 {
			add(m[1], true)
		}
	}
	for _, m := range reVideo.FindAllStringSubmatch(page, -1) {
		if len(m) > 1 {
			add(m[1], true)
		}
	}

	// Regex sweep as fallback — filtered aggressively.
	for _, m := range reM3U8.FindAllString(page, -1) {
		add(m, false)
	}
	for _, m := range reMP4.FindAllString(page, -1) {
		add(m, false)
	}
	for _, m := range reWebM.FindAllString(page, -1) {
		add(m, false)
	}

	return cands, title, nil
}

func firstMatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func absURL(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	return u.String()
}

func cleanURL(s string) string {
	s = strings.TrimSpace(s)
	s = html.UnescapeString(s)
	s = strings.TrimRight(s, `",');>\`)
	// Truncate anything after a media extension (handles JSON/JS bleed).
	lower := strings.ToLower(s)
	for _, ext := range []string{".m3u8", ".mp4", ".webm"} {
		if i := strings.Index(lower, ext); i >= 0 {
			end := i + len(ext)
			rest := s[end:]
			if rest == "" {
				return s[:end]
			}
			if rest[0] == '?' {
				// Keep query until delimiter that is not valid in query.
				qEnd := len(s)
				for j, r := range rest[1:] {
					if r == '"' || r == '\'' || r == '<' || r == '>' || r == ' ' || r == '\n' || r == '\r' || r == '\\' {
						qEnd = end + 1 + j
						break
					}
					// Comma / brace often means JSON continuation, not query.
					if r == ',' || r == '{' || r == '}' || r == '[' || r == ']' {
						qEnd = end + 1 + j
						break
					}
				}
				return s[:qEnd]
			}
			return s[:end]
		}
	}
	return s
}

func isPlausibleMediaURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "&quot;") || strings.Contains(lower, "%22") {
		return false
	}
	// Reject if path still looks like concatenated junk after extension.
	p := strings.ToLower(u.Path)
	extOK := strings.HasSuffix(p, ".mp4") || strings.HasSuffix(p, ".m3u8") || strings.HasSuffix(p, ".webm") ||
		strings.Contains(p, ".mp4/") || strings.Contains(p, ".m3u8/") // rare CDN style
	if !extOK {
		// Allow query-based streams without extension when marked HLS-ish.
		if strings.Contains(lower, "m3u8") || strings.Contains(lower, "mpegurl") {
			return true
		}
		return false
	}
	return true
}

func isStrongNoiseMediaURL(raw string) bool {
	lower := strings.ToLower(raw)
	strong := []string{
		"/heat-preview/",
		"fh_heatmap_preview",
		"heatmap_preview",
		"/sprite",
		"/sprites/",
	}
	for _, n := range strong {
		if strings.Contains(lower, n) {
			return true
		}
	}
	return false
}

func isNoiseMediaURL(raw string) bool {
	lower := strings.ToLower(raw)
	u, err := url.Parse(raw)
	if err == nil {
		host := strings.ToLower(u.Host)
		// Thumbnail / preview CDN hostnames.
		if strings.HasPrefix(host, "thumb.") || strings.HasPrefix(host, "thumbs.") ||
			strings.HasPrefix(host, "thumb-") || strings.Contains(host, ".thumb.") {
			return true
		}
		base := path.Base(u.Path)
		if strings.HasPrefix(base, "sprite") || strings.Contains(base, "poster") {
			return true
		}
	}
	for _, n := range noisePathFragments {
		if strings.Contains(lower, n) {
			return true
		}
	}
	return false
}

func guessExt(u string) string {
	lower := strings.ToLower(u)
	switch {
	case strings.Contains(lower, ".m3u8"):
		return "m3u8"
	case strings.Contains(lower, ".mp4"):
		return "mp4"
	case strings.Contains(lower, ".webm"):
		return "webm"
	default:
		return ""
	}
}
