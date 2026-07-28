package download

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrBlockedURL is returned when a URL fails SSRF / scheme checks.
var ErrBlockedURL = fmt.Errorf("url blocked")

// ValidateURL parses and checks scheme + host (no private IP until ResolveAndCheck).
func ValidateURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("%w: empty", ErrBlockedURL)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBlockedURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("%w: scheme %s", ErrBlockedURL, u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("%w: missing host", ErrBlockedURL)
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("%w: missing hostname", ErrBlockedURL)
	}
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return nil, fmt.Errorf("%w: localhost", ErrBlockedURL)
	}
	if ip := net.ParseIP(host); ip != nil && isBlockedIP(ip) {
		return nil, fmt.Errorf("%w: private ip", ErrBlockedURL)
	}
	return u, nil
}

// ResolveAndCheck resolves DNS and rejects private/link-local/metadata addresses.
func ResolveAndCheck(ctx context.Context, u *url.URL) error {
	host := u.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("%w: private ip", ErrBlockedURL)
		}
		return nil
	}
	resolver := net.DefaultResolver
	addrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("dns lookup: %w", err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("%w: no addresses", ErrBlockedURL)
	}
	for _, a := range addrs {
		if isBlockedIP(a.IP) {
			return fmt.Errorf("%w: resolves to private ip", ErrBlockedURL)
		}
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
	}
	return false
}

// SafeHTTPClient returns an HTTP client that re-validates redirects against SSRF rules.
// Uses HTTP/1.1 (no HTTP/2) to avoid CDN multiplex resets taking down many streams at once.
// timeout <= 0 means no overall Client.Timeout (for large file downloads); dial/header
// timeouts still apply via Transport. Pass a positive duration for short probes (analyze/HEAD).
func SafeHTTPClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     false,
		TLSNextProto:          map[string]func(authority string, c *tls.Conn) http.RoundTripper{},
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	c := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			u, err := ValidateURL(req.URL.String())
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
			defer cancel()
			return ResolveAndCheck(ctx, u)
		},
	}
	if timeout > 0 {
		c.Timeout = timeout
	}
	return c
}

// RedactURL strips query values for safe logging.
func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "[invalid-url]"
	}
	u.RawQuery = ""
	u.Fragment = ""
	if u.User != nil {
		u.User = url.User("redacted")
	}
	return u.String()
}
