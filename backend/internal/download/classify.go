package download

import (
	"context"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
)

const (
	KindDirect = "direct"
	KindHLS    = "hls"
	KindYTDLP  = "ytdlp"
	KindHTML   = "html"
	KindWebsite = "website" // analyze-only; enqueue uses ytdlp/html
)

// ClassifyResult is the outcome of URL classification.
type ClassifyResult struct {
	Kind        string `json:"kind"` // direct | hls | website
	ContentType string `json:"content_type,omitempty"`
	Filename    string `json:"filename,omitempty"`
	FinalURL    string `json:"final_url,omitempty"`
}

var videoExts = map[string]bool{
	".mp4": true, ".mkv": true, ".webm": true, ".mov": true,
	".m4v": true, ".avi": true, ".ts": true, ".flv": true,
	".mpg": true, ".mpeg": true, ".wmv": true, ".3gp": true,
}

// Classify inspects a URL via path heuristics and a safe HEAD/GET.
func Classify(ctx context.Context, rawURL string, analyzeTimeout time.Duration) (*ClassifyResult, error) {
	u, err := ValidateURL(rawURL)
	if err != nil {
		return nil, err
	}
	if err := ResolveAndCheck(ctx, u); err != nil {
		return nil, err
	}

	ext := strings.ToLower(path.Ext(u.Path))
	if ext == ".m3u8" {
		return &ClassifyResult{Kind: KindHLS, Filename: path.Base(u.Path), FinalURL: u.String()}, nil
	}
	if videoExts[ext] {
		return &ClassifyResult{Kind: KindDirect, Filename: path.Base(u.Path), FinalURL: u.String()}, nil
	}

	client := SafeHTTPClient(analyzeTimeout)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", browserUA)

	resp, err := client.Do(req)
	if err != nil {
		// Some servers reject HEAD — fall through to website analyze.
		return &ClassifyResult{Kind: KindWebsite, FinalURL: u.String()}, nil
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))

	ct := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	finalURL := u.String()
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	fname := filenameFromHeaders(resp.Header, finalURL)

	switch {
	case strings.Contains(ct, "mpegurl") || strings.HasSuffix(strings.ToLower(fname), ".m3u8"):
		return &ClassifyResult{Kind: KindHLS, ContentType: ct, Filename: fname, FinalURL: finalURL}, nil
	case strings.HasPrefix(ct, "video/") || videoExts[strings.ToLower(path.Ext(fname))]:
		return &ClassifyResult{Kind: KindDirect, ContentType: ct, Filename: fname, FinalURL: finalURL}, nil
	case strings.HasPrefix(ct, "application/octet-stream") && videoExts[strings.ToLower(path.Ext(fname))]:
		return &ClassifyResult{Kind: KindDirect, ContentType: ct, Filename: fname, FinalURL: finalURL}, nil
	default:
		return &ClassifyResult{Kind: KindWebsite, ContentType: ct, Filename: fname, FinalURL: finalURL}, nil
	}
}

func filenameFromHeaders(h http.Header, finalURL string) string {
	cd := h.Get("Content-Disposition")
	if cd != "" {
		lower := strings.ToLower(cd)
		if i := strings.Index(lower, "filename="); i >= 0 {
			v := strings.TrimSpace(cd[i+len("filename="):])
			v = strings.Trim(v, `"'`)
			if v != "" {
				return path.Base(v)
			}
		}
	}
	if u, err := ValidateURL(finalURL); err == nil {
		base := path.Base(u.Path)
		if base != "" && base != "/" && base != "." {
			return base
		}
	}
	return "download"
}

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
