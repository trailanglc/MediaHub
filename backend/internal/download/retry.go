package download

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

const defaultFetchAttempts = 5

// IsTransient reports whether err is worth retrying (CDN reset, timeout, 5xx/429).
func IsTransient(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && (ne.Timeout() || ne.Temporary()) {
		return true
	}
	var op *net.OpError
	if errors.As(err, &op) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNABORTED) || errors.Is(err, syscall.ETIMEDOUT) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, needle := range []string{
		"connection reset",
		"broken pipe",
		"i/o timeout",
		"tls handshake timeout",
		"server closed idle connection",
		"http2: client connection force closed",
		"stream error",
		"temporary failure",
		"timeout",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

func isRetryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable,
		http.StatusGatewayTimeout, http.StatusInternalServerError, 520, 521, 522, 523, 524:
		return true
	default:
		return false
	}
}

// WithRetry runs fn up to attempts times with exponential backoff on transient errors.
func WithRetry(ctx context.Context, attempts int, fn func() error) error {
	if attempts < 1 {
		attempts = defaultFetchAttempts
	}
	var last error
	for i := 0; i < attempts; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		last = fn()
		if last == nil {
			return nil
		}
		if !IsTransient(last) && !isHTTPRetryable(last) {
			return last
		}
		if i == attempts-1 {
			break
		}
		delay := time.Duration(1<<uint(i)) * 400 * time.Millisecond
		if delay > 8*time.Second {
			delay = 8 * time.Second
		}
		// jitter-ish
		delay += time.Duration(i*50) * time.Millisecond
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return last
}

type httpStatusError struct {
	code int
	msg  string
}

func (e *httpStatusError) Error() string {
	return e.msg
}

func isHTTPRetryable(err error) bool {
	var he *httpStatusError
	if errors.As(err, &he) {
		return isRetryableStatus(he.code)
	}
	return false
}
