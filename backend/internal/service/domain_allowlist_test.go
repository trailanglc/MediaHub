package service

import "testing"

func TestMatchDomainAllowlist_exactHost(t *testing.T) {
	if !MatchDomainAllowlist("https://app.example.com", []string{"app.example.com"}, nil) {
		t.Fatal("expected exact host match")
	}
	if MatchDomainAllowlist("https://evil.example.com", []string{"example.com"}, nil) {
		t.Fatal("apex entry must not match evil.example.com")
	}
	if !MatchDomainAllowlist("https://example.com", []string{"example.com"}, nil) {
		t.Fatal("expected apex match")
	}
}

func TestMatchDomainAllowlist_subdomainSuffix(t *testing.T) {
	if !MatchDomainAllowlist("https://cdn.app.example.com", []string{".example.com"}, nil) {
		t.Fatal("expected .example.com suffix match")
	}
	if MatchDomainAllowlist("https://notexample.com", []string{".example.com"}, nil) {
		t.Fatal("should not match unrelated host")
	}
}

func TestMatchDomainAllowlist_emptyAndWildcard(t *testing.T) {
	if MatchDomainAllowlist("https://any.com", []string{}, nil) {
		t.Fatal("empty allowlist should deny")
	}
	if MatchDomainAllowlist("https://any.com", []string{"*"}, nil) {
		t.Fatal("wildcard entry should be ignored")
	}
	if MatchDomainAllowlist("", []string{"example.com"}, nil) {
		t.Fatal("empty origin should fail")
	}
}

func TestMatchDomainAllowlist_refererAndBareHost(t *testing.T) {
	if !MatchDomainAllowlist("https://player.example.com/watch?v=1", []string{"player.example.com"}, nil) {
		t.Fatal("expected referer host parse")
	}
	if !MatchDomainAllowlist("localhost:3000", []string{"localhost"}, nil) {
		t.Fatal("expected bare host with port")
	}
}
