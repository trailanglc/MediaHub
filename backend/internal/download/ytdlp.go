package download

	import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Candidate is a selectable media URL from analyze.
type Candidate struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	URL       string `json:"url,omitempty"`
	Ext       string `json:"ext,omitempty"`
	Height    int    `json:"height,omitempty"`
	Width     int    `json:"width,omitempty"`
	FPS       float64 `json:"fps,omitempty"`
	Filesize  int64  `json:"filesize,omitempty"`
	Thumbnail string `json:"thumbnail,omitempty"`
	IsHLS     bool   `json:"is_hls"`
	FormatNote string `json:"format_note,omitempty"`
	Protocol  string `json:"protocol,omitempty"`
}

type ytdlpInfo struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Thumbnail   string         `json:"thumbnail"`
	Thumbnails  []ytdlpThumb   `json:"thumbnails"`
	Formats     []ytdlpFormat  `json:"formats"`
	URL         string         `json:"url"`
	Ext         string         `json:"ext"`
	IsLive      bool           `json:"is_live"`
}

type ytdlpThumb struct {
	URL string `json:"url"`
}

type ytdlpFormat struct {
	FormatID   string  `json:"format_id"`
	URL        string  `json:"url"`
	Ext        string  `json:"ext"`
	Height     int     `json:"height"`
	Width      int     `json:"width"`
	FPS        float64 `json:"fps"`
	Filesize   int64   `json:"filesize"`
	FilesizeApprox int64 `json:"filesize_approx"`
	FormatNote string  `json:"format_note"`
	Protocol   string  `json:"protocol"`
	VCodec     string  `json:"vcodec"`
	ACodec     string  `json:"acodec"`
	ManifestURL string `json:"manifest_url"`
}

// AnalyzeYTDLP runs yt-dlp -J without downloading.
func AnalyzeYTDLP(ctx context.Context, ytdlpPath, rawURL string, timeout time.Duration) ([]Candidate, string, error) {
	if ytdlpPath == "" {
		ytdlpPath = "yt-dlp"
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, ytdlpPath,
		"-J", "--no-download", "--no-warnings", "--flat-playlist",
		"--no-check-certificates",
		rawURL,
	)
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = ee.Stderr
		}
		return nil, "", fmt.Errorf("yt-dlp: %w: %s", err, strings.TrimSpace(string(stderr)))
	}

	var info ytdlpInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, "", fmt.Errorf("yt-dlp json: %w", err)
	}

	thumb := info.Thumbnail
	if thumb == "" && len(info.Thumbnails) > 0 {
		thumb = info.Thumbnails[len(info.Thumbnails)-1].URL
	}
	title := strings.TrimSpace(info.Title)
	if title == "" {
		title = "Video"
	}

	var cands []Candidate
	seen := map[string]bool{}
	for _, f := range info.Formats {
		mediaURL := strings.TrimSpace(f.URL)
		if mediaURL == "" {
			mediaURL = strings.TrimSpace(f.ManifestURL)
		}
		if mediaURL == "" {
			continue
		}
		// Prefer formats with video (or HLS manifests).
		proto := strings.ToLower(f.Protocol)
		isHLS := strings.Contains(proto, "m3u8") ||
			strings.HasSuffix(strings.ToLower(mediaURL), ".m3u8") ||
			strings.EqualFold(f.Ext, "m3u8")
		hasVideo := f.VCodec != "" && f.VCodec != "none"
		if !hasVideo && !isHLS {
			continue
		}
		key := f.FormatID + "|" + mediaURL
		if seen[key] {
			continue
		}
		seen[key] = true
		size := f.Filesize
		if size == 0 {
			size = f.FilesizeApprox
		}
		cands = append(cands, Candidate{
			ID:         f.FormatID,
			Title:      title,
			URL:        mediaURL,
			Ext:        f.Ext,
			Height:     f.Height,
			Width:      f.Width,
			FPS:        f.FPS,
			Filesize:   size,
			Thumbnail:  thumb,
			IsHLS:      isHLS,
			FormatNote: f.FormatNote,
			Protocol:   f.Protocol,
		})
	}

	// Single-format pages (direct url on info).
	if len(cands) == 0 && info.URL != "" {
		isHLS := strings.HasSuffix(strings.ToLower(info.URL), ".m3u8") || strings.EqualFold(info.Ext, "m3u8")
		cands = append(cands, Candidate{
			ID:        "0",
			Title:     title,
			URL:       info.URL,
			Ext:       info.Ext,
			Thumbnail: thumb,
			IsHLS:     isHLS,
		})
	}

	return cands, title, nil
}

// DownloadYTDLPFile downloads a specific format to destPath (merged progressive file).
// maxBytes > 0 is passed as --max-filesize (yt-dlp aborts oversized downloads).
func DownloadYTDLPFile(ctx context.Context, ytdlpPath, rawURL, formatID, destPath string, maxBytes int64) error {
	if ytdlpPath == "" {
		ytdlpPath = "yt-dlp"
	}
	args := []string{
		"-o", destPath,
		"--no-warnings",
		"--no-playlist",
		"--newline",
	}
	if maxBytes > 0 {
		args = append(args, "--max-filesize", strconv.FormatInt(maxBytes, 10))
	}
	if formatID != "" {
		args = append(args, "-f", formatID)
	}
	args = append(args, rawURL)
	cmd := exec.CommandContext(ctx, ytdlpPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("yt-dlp download: %w: %s", err, truncate(string(out), 500))
	}
	return nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
