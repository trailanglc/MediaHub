package download

import (
	"context"
	"errors"
	"io"
	"net"
	"syscall"
	"testing"
	"time"
)

func TestIsTransient_ConnectionReset(t *testing.T) {
	err := &net.OpError{Op: "read", Err: syscall.ECONNRESET}
	if !IsTransient(err) {
		t.Fatal("expected transient for ECONNRESET")
	}
	if IsTransient(context.Canceled) {
		t.Fatal("canceled should not retry")
	}
	if !IsTransient(io.ErrUnexpectedEOF) {
		t.Fatal("unexpected EOF should retry")
	}
}

func TestWithRetry_SucceedsAfterTransient(t *testing.T) {
	ctx := context.Background()
	n := 0
	err := WithRetry(ctx, 4, func() error {
		n++
		if n < 3 {
			return errors.New("read: connection reset by peer")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("attempts=%d", n)
	}
}

func TestWithRetry_RespectsCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	err := WithRetry(ctx, 10, func() error {
		return errors.New("connection reset by peer")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestHTTPStatusRetryable(t *testing.T) {
	err := &httpStatusError{code: 429, msg: "http 429"}
	if !isHTTPRetryable(err) {
		t.Fatal("429 should retry")
	}
	err2 := &httpStatusError{code: 404, msg: "http 404"}
	if isHTTPRetryable(err2) {
		t.Fatal("404 should not retry")
	}
}
