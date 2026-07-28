package download

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	maxPlaylistBytes = 32 << 20 // 32 MiB
	maxSegmentBytes  = 64 << 20 // 64 MiB per segment
)

// HLSMirrorResult describes mirrored local HLS tree under OutDir.
type HLSMirrorResult struct {
	OutDir     string
	MasterRel  string // relative path e.g. master.m3u8
	FileCount  int
	BytesTotal int64
}

var reBandwidth = regexp.MustCompile(`(?i)BANDWIDTH=(\d+)`)

// MirrorHLS downloads playlists + segments and rewrites URIs to relative paths under outDir.
// For master playlists only the highest-BANDWIDTH variant is mirrored (disk/OOM safety).
// maxBytes caps total mirrored bytes (0 = unlimited).
func MirrorHLS(ctx context.Context, client *http.Client, playlistURL, outDir string, concurrency int, maxBytes int64, onProgress ProgressFunc) (*HLSMirrorResult, error) {
	if client == nil {
		client = SafeHTTPClient(0)
	}
	if concurrency < 1 {
		concurrency = 2
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}

	u, err := ValidateURL(playlistURL)
	if err != nil {
		return nil, err
	}
	if err := ResolveAndCheck(ctx, u); err != nil {
		return nil, err
	}

	raw, finalURL, err := fetchBytes(ctx, client, u.String())
	if err != nil {
		return nil, err
	}
	base, _ := url.Parse(finalURL)
	lines := splitLines(string(raw))

	if looksLikeMaster(lines) {
		return mirrorMaster(ctx, client, base, lines, outDir, concurrency, maxBytes, onProgress)
	}
	return mirrorMedia(ctx, client, base, lines, outDir, "master.m3u8", concurrency, maxBytes, onProgress)
}

func looksLikeMaster(lines []string) bool {
	for _, ln := range lines {
		if strings.HasPrefix(ln, "#EXT-X-STREAM-INF") {
			return true
		}
	}
	return false
}

func parseBandwidth(infLine string) int64 {
	m := reBandwidth.FindStringSubmatch(infLine)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.ParseInt(m[1], 10, 64)
	return n
}

func mirrorMaster(ctx context.Context, client *http.Client, base *url.URL, lines []string, outDir string, concurrency int, maxBytes int64, onProgress ProgressFunc) (*HLSMirrorResult, error) {
	type variant struct {
		infLine string
		uri     string
		bw      int64
	}
	var variants []variant
	for i := 0; i < len(lines); i++ {
		ln := lines[i]
		if !strings.HasPrefix(ln, "#EXT-X-STREAM-INF") {
			continue
		}
		uri := ""
		if i+1 < len(lines) && !strings.HasPrefix(lines[i+1], "#") {
			uri = strings.TrimSpace(lines[i+1])
			i++
		}
		if uri == "" {
			continue
		}
		variants = append(variants, variant{infLine: ln, uri: uri, bw: parseBandwidth(ln)})
	}
	if hasEncryption(lines) {
		return nil, fmt.Errorf("encrypted HLS is not supported")
	}
	if len(variants) == 0 {
		return nil, fmt.Errorf("master playlist has no variants")
	}

	// Pick highest bandwidth only — mirroring every rendition can fill disk.
	best := variants[0]
	for _, v := range variants[1:] {
		if v.bw > best.bw {
			best = v
		}
	}

	abs := resolveRef(base, best.uri)
	mediaRaw, mediaFinal, err := fetchBytes(ctx, client, abs)
	if err != nil {
		return nil, fmt.Errorf("variant: %w", err)
	}
	mediaBase, _ := url.Parse(mediaFinal)
	mediaLines := splitLines(string(mediaRaw))
	if hasEncryption(mediaLines) {
		return nil, fmt.Errorf("encrypted HLS is not supported")
	}

	dir := filepath.Join(outDir, "v0")
	res, err := mirrorMedia(ctx, client, mediaBase, mediaLines, dir, "index.m3u8", concurrency, maxBytes, onProgress)
	if err != nil {
		return nil, err
	}

	masterLines := []string{"#EXTM3U", best.infLine, path.Join("v0", "index.m3u8")}
	masterPath := filepath.Join(outDir, "master.m3u8")
	if err := os.WriteFile(masterPath, []byte(strings.Join(masterLines, "\n")+"\n"), 0o644); err != nil {
		return nil, err
	}
	if onProgress != nil {
		onProgress(res.BytesTotal, res.BytesTotal)
	}
	return &HLSMirrorResult{
		OutDir:     outDir,
		MasterRel:  "master.m3u8",
		FileCount:  res.FileCount + 1,
		BytesTotal: res.BytesTotal,
	}, nil
}

func mirrorMedia(ctx context.Context, client *http.Client, base *url.URL, lines []string, outDir, playlistName string, concurrency int, maxBytes int64, onProgress ProgressFunc) (*HLSMirrorResult, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	if hasEncryption(lines) {
		return nil, fmt.Errorf("encrypted HLS is not supported")
	}

	type seg struct {
		absURL string
		name   string
	}
	var segs []seg
	var outLines []string
	segIdx := 0
	for _, ln := range lines {
		if ln == "" || strings.HasPrefix(ln, "#") {
			outLines = append(outLines, ln)
			continue
		}
		abs := resolveRef(base, strings.TrimSpace(ln))
		ext := path.Ext(strings.Split(path.Base(abs), "?")[0])
		if ext == "" {
			ext = ".ts"
		}
		name := fmt.Sprintf("segment_%05d%s", segIdx, ext)
		segIdx++
		outLines = append(outLines, name)
		segs = append(segs, seg{absURL: abs, name: name})
	}

	var bytesTotal atomic.Int64
	var firstErrMu sync.Mutex
	var firstErr error
	var doneCount atomic.Int32
	totalSegs := int32(len(segs))

	dlCtx, dlCancel := context.WithCancel(ctx)
	defer dlCancel()

	setErr := func(err error) {
		if err == nil {
			return
		}
		firstErrMu.Lock()
		if firstErr == nil {
			firstErr = err
			dlCancel()
		}
		firstErrMu.Unlock()
	}

	jobs := make(chan seg)
	var wg sync.WaitGroup
	workers := concurrency
	if workers > len(segs) {
		workers = len(segs)
	}
	if workers < 1 && len(segs) > 0 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for s := range jobs {
				if dlCtx.Err() != nil {
					continue
				}
				n, err := downloadFileToDisk(dlCtx, client, s.absURL, filepath.Join(outDir, s.name), maxSegmentBytes)
				if err != nil {
					setErr(err)
					continue
				}
				cur := bytesTotal.Add(n)
				if maxBytes > 0 && cur > maxBytes {
					setErr(fmt.Errorf("hls exceeds max size (%d > %d)", cur, maxBytes))
					continue
				}
				c := doneCount.Add(1)
				if onProgress != nil {
					var estTotal int64
					if c > 0 && totalSegs > 0 {
						estTotal = cur * int64(totalSegs) / int64(c)
					}
					onProgress(cur, estTotal)
				}
			}
		}()
	}

	for _, s := range segs {
		if dlCtx.Err() != nil {
			break
		}
		select {
		case <-dlCtx.Done():
		case jobs <- s:
		}
	}
	close(jobs)
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	plPath := filepath.Join(outDir, playlistName)
	if err := os.WriteFile(plPath, []byte(strings.Join(outLines, "\n")+"\n"), 0o644); err != nil {
		return nil, err
	}
	return &HLSMirrorResult{
		OutDir:     outDir,
		MasterRel:  playlistName,
		FileCount:  len(segs) + 1,
		BytesTotal: bytesTotal.Load(),
	}, nil
}

func hasEncryption(lines []string) bool {
	for _, ln := range lines {
		upper := strings.ToUpper(ln)
		if strings.HasPrefix(upper, "#EXT-X-KEY:") && !strings.Contains(upper, "METHOD=NONE") {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	sc := bufio.NewScanner(strings.NewReader(s))
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		lines = append(lines, strings.TrimRight(sc.Text(), "\r"))
	}
	return lines
}

func resolveRef(base *url.URL, ref string) string {
	u, err := url.Parse(strings.TrimSpace(ref))
	if err != nil {
		return ref
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	return u.String()
}

func fetchBytes(ctx context.Context, client *http.Client, rawURL string) ([]byte, string, error) {
	var out []byte
	var final string
	err := WithRetry(ctx, defaultFetchAttempts, func() error {
		b, f, e := fetchBytesOnce(ctx, client, rawURL)
		if e != nil {
			return e
		}
		out, final = b, f
		return nil
	})
	return out, final, err
}

func fetchBytesOnce(ctx context.Context, client *http.Client, rawURL string) ([]byte, string, error) {
	u, err := ValidateURL(rawURL)
	if err != nil {
		return nil, "", err
	}
	if err := ResolveAndCheck(ctx, u); err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "*/*")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, "", &httpStatusError{
			code: resp.StatusCode,
			msg:  fmt.Sprintf("http %d", resp.StatusCode),
		}
	}
	limited := io.LimitReader(resp.Body, maxPlaylistBytes+1)
	b, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", err
	}
	if int64(len(b)) > maxPlaylistBytes {
		return nil, "", fmt.Errorf("playlist exceeds max size")
	}
	final := u.String()
	if resp.Request != nil && resp.Request.URL != nil {
		final = resp.Request.URL.String()
	}
	return b, final, nil
}

// downloadFileToDisk streams a segment to disk without loading the whole body into RAM.
func downloadFileToDisk(ctx context.Context, client *http.Client, rawURL, dest string, maxSeg int64) (int64, error) {
	var n int64
	err := WithRetry(ctx, defaultFetchAttempts, func() error {
		written, e := downloadFileToDiskOnce(ctx, client, rawURL, dest, maxSeg)
		if e != nil {
			_ = os.Remove(dest)
			return e
		}
		n = written
		return nil
	})
	return n, err
}

func downloadFileToDiskOnce(ctx context.Context, client *http.Client, rawURL, dest string, maxSeg int64) (int64, error) {
	u, err := ValidateURL(rawURL)
	if err != nil {
		return 0, err
	}
	if err := ResolveAndCheck(ctx, u); err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "*/*")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return 0, &httpStatusError{
			code: resp.StatusCode,
			msg:  fmt.Sprintf("http %d", resp.StatusCode),
		}
	}
	if maxSeg <= 0 {
		maxSeg = maxSegmentBytes
	}
	f, err := os.Create(dest)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	written, err := io.Copy(f, io.LimitReader(resp.Body, maxSeg+1))
	if err != nil {
		return written, err
	}
	if written > maxSeg {
		return written, fmt.Errorf("segment exceeds max size (%d > %d)", written, maxSeg)
	}
	return written, nil
}
