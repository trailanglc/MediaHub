package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// ProgressFunc reports download progress.
type ProgressFunc func(done, total int64)

// FetchToFile downloads url to destPath, using concurrent Range requests when possible.
func FetchToFile(ctx context.Context, client *http.Client, rawURL, destPath string, concurrency int, maxBytes int64, onProgress ProgressFunc) (contentType string, size int64, err error) {
	if client == nil {
		client = SafeHTTPClient(0)
	}
	if concurrency < 1 {
		concurrency = 4
	}

	u, err := ValidateURL(rawURL)
	if err != nil {
		return "", 0, err
	}
	if err := ResolveAndCheck(ctx, u); err != nil {
		return "", 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Range", "bytes=0-0")

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
	resp.Body.Close()
	ct := resp.Header.Get("Content-Type")

	var total int64 = -1
	acceptRanges := resp.Header.Get("Accept-Ranges") == "bytes" || resp.StatusCode == http.StatusPartialContent
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		// bytes 0-0/12345
		var start, end, n int64
		if _, scanErr := fmt.Sscanf(cr, "bytes %d-%d/%d", &start, &end, &n); scanErr == nil && n > 0 {
			total = n
		}
	}
	if total < 0 && resp.ContentLength > 1 {
		total = resp.ContentLength
	}
	_ = body

	if maxBytes > 0 && total > maxBytes {
		return ct, 0, fmt.Errorf("file exceeds max size (%d > %d)", total, maxBytes)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return ct, 0, err
	}

	// Single-stream fallback when size unknown or ranges unsupported.
	if !acceptRanges || total <= 0 || concurrency == 1 || total < 1<<20 {
		return fetchSingle(ctx, client, u.String(), destPath, maxBytes, onProgress)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return ct, 0, err
	}
	defer f.Close()
	if err := f.Truncate(total); err != nil {
		return ct, 0, err
	}

	chunkSize := total / int64(concurrency)
	if chunkSize < 1<<20 {
		chunkSize = 1 << 20
	}
	type span struct{ start, end int64 }
	var spans []span
	for start := int64(0); start < total; start += chunkSize {
		end := start + chunkSize - 1
		if end >= total {
			end = total - 1
		}
		spans = append(spans, span{start, end})
	}

	var done atomic.Int64
	var firstErrMu sync.Mutex
	var firstErr error
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, sp := range spans {
		sp := sp
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ctx.Err() != nil {
				return
			}
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := fetchRange(ctx, client, u.String(), destPath, sp.start, sp.end); err != nil {
				firstErrMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				firstErrMu.Unlock()
				return
			}
			n := sp.end - sp.start + 1
			cur := done.Add(n)
			if onProgress != nil {
				onProgress(cur, total)
			}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		_ = os.Remove(destPath)
		return ct, 0, firstErr
	}
	if onProgress != nil {
		onProgress(total, total)
	}
	return ct, total, nil
}

func fetchSingle(ctx context.Context, client *http.Client, rawURL, destPath string, maxBytes int64, onProgress ProgressFunc) (string, int64, error) {
	var ct string
	var written int64
	err := WithRetry(ctx, defaultFetchAttempts, func() error {
		c, n, e := fetchSingleOnce(ctx, client, rawURL, destPath, maxBytes, onProgress)
		if e != nil {
			_ = os.Remove(destPath)
			return e
		}
		ct, written = c, n
		return nil
	})
	return ct, written, err
}

func fetchSingleOnce(ctx context.Context, client *http.Client, rawURL, destPath string, maxBytes int64, onProgress ProgressFunc) (string, int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "*/*")
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return "", 0, &httpStatusError{
			code: resp.StatusCode,
			msg:  fmt.Sprintf("http %d", resp.StatusCode),
		}
	}
	ct := resp.Header.Get("Content-Type")
	if maxBytes > 0 && resp.ContentLength > maxBytes {
		return ct, 0, fmt.Errorf("file exceeds max size")
	}
	f, err := os.Create(destPath)
	if err != nil {
		return ct, 0, err
	}
	defer f.Close()

	var written int64
	buf := make([]byte, 256<<10)
	for {
		nr, er := resp.Body.Read(buf)
		if nr > 0 {
			if maxBytes > 0 && written+int64(nr) > maxBytes {
				return ct, written, fmt.Errorf("file exceeds max size")
			}
			nw, ew := f.Write(buf[:nr])
			written += int64(nw)
			if ew != nil {
				return ct, written, ew
			}
			if onProgress != nil {
				total := resp.ContentLength
				if total <= 0 {
					total = written
				}
				onProgress(written, total)
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			return ct, written, er
		}
		if ctx.Err() != nil {
			return ct, written, ctx.Err()
		}
	}
	return ct, written, nil
}

func fetchRange(ctx context.Context, client *http.Client, rawURL, destPath string, start, end int64) error {
	return WithRetry(ctx, defaultFetchAttempts, func() error {
		return fetchRangeOnce(ctx, client, rawURL, destPath, start, end)
	})
}

func fetchRangeOnce(ctx context.Context, client *http.Client, rawURL, destPath string, start, end int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	want := end - start + 1
	if want < 1 {
		return fmt.Errorf("invalid range")
	}
	if resp.StatusCode != http.StatusPartialContent {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return &httpStatusError{
			code: resp.StatusCode,
			msg:  fmt.Sprintf("range http %d (expected 206)", resp.StatusCode),
		}
	}
	f, err := os.OpenFile(destPath, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(resp.Body, want))
	if err != nil {
		return err
	}
	if n != want {
		return fmt.Errorf("range short read: got %d want %d", n, want)
	}
	return nil
}
