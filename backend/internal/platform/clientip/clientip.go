package clientip

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

// FromGin returns the normalized client IP from a Gin request context.
// Requires trusted reverse proxies to be configured on the Gin engine.
func FromGin(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	return Normalize(c.ClientIP())
}

// Normalize canonicalizes IPv4/IPv6 strings for consistent audit storage and display.
// Handles comma-separated X-Forwarded-For values and IPv4-mapped IPv6 addresses.
func Normalize(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if idx := strings.Index(raw, ","); idx >= 0 {
		raw = strings.TrimSpace(raw[:idx])
	}

	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")

	if zone := strings.Index(raw, "%"); zone >= 0 {
		raw = raw[:zone]
	}

	ip := net.ParseIP(raw)
	if ip == nil {
		return raw
	}

	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}

	return ip.String()
}
