package service

import (
	"net/url"
	"strings"
)

// MatchDomainAllowlist returns true when origin/referer host matches an entry in allowed or global.
// Empty allowlists deny (embed must use APP_URL or authenticated stream path).
// Wildcard "*" and empty entries are ignored. Host matching is exact or subdomain via ".suffix" entries.
func MatchDomainAllowlist(origin string, allowed []string, global []string) bool {
	all := append(allowed, global...)
	if len(all) == 0 {
		return false
	}
	host := originHost(origin)
	if host == "" {
		return false
	}
	for _, d := range all {
		entry := normalizeAllowlistHost(d)
		if entry == "" || entry == "*" {
			continue
		}
		if hostMatchesAllowlistEntry(host, entry) {
			return true
		}
	}
	return false
}

func originHost(originOrReferer string) string {
	s := strings.TrimSpace(strings.ToLower(originOrReferer))
	if s == "" {
		return ""
	}
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil || u.Host == "" {
			return ""
		}
		return stripPort(u.Host)
	}
	// Bare host or host:port
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}
	return stripPort(s)
}

func stripPort(host string) string {
	if host == "" {
		return ""
	}
	if h, _, err := netSplitHostPort(host); err == nil {
		return strings.ToLower(h)
	}
	return strings.ToLower(host)
}

func netSplitHostPort(hostport string) (host, port string, err error) {
	// Avoid importing net only for SplitHostPort on IPv6; use url for common cases.
	if strings.HasPrefix(hostport, "[") {
		if i := strings.LastIndex(hostport, "]:"); i >= 0 {
			return hostport[1:i], hostport[i+2:], nil
		}
		if strings.HasSuffix(hostport, "]") {
			return hostport[1 : len(hostport)-1], "", nil
		}
	}
	if i := strings.LastIndex(hostport, ":"); i >= 0 && strings.Count(hostport, ":") == 1 {
		return hostport[:i], hostport[i+1:], nil
	}
	return hostport, "", nil
}

func normalizeAllowlistHost(entry string) string {
	entry = strings.TrimSpace(strings.ToLower(entry))
	if entry == "" || entry == "*" {
		return ""
	}
	dotSuffix := strings.HasPrefix(entry, ".")
	if strings.Contains(entry, "://") {
		h := originHost(entry)
		if h == "" {
			return ""
		}
		if dotSuffix {
			return "." + h
		}
		return h
	}
	if idx := strings.Index(entry, "/"); idx >= 0 {
		entry = entry[:idx]
	}
	host := stripPort(strings.TrimPrefix(entry, "."))
	if host == "" {
		return ""
	}
	if dotSuffix {
		return "." + host
	}
	return host
}

// hostMatchesAllowlistEntry matches host against entry.
// entry "example.com" matches only example.com (not evil.example.com).
// entry ".example.com" or allowlist stored as "example.com" with subdomain: use leading dot in config
// or exact apex — we treat entry with leading dot in original allowed list via normalize keeping dot prefix.
func hostMatchesAllowlistEntry(host, entry string) bool {
	if host == "" || entry == "" {
		return false
	}
	if strings.HasPrefix(entry, ".") {
		suffix := entry
		if host == suffix[1:] {
			return true
		}
		return strings.HasSuffix(host, suffix)
	}
	return host == entry
}
