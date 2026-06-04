package service

import (
	"testing"

	"github.com/anhtuanlc/mediahub/internal/integration"
	"github.com/anhtuanlc/mediahub/internal/repository"
)

func TestAPIKeyService_MatchIP(t *testing.T) {
	s := &APIKeyService{}
	if !s.MatchIP("1.2.3.4", nil) {
		t.Fatal("empty allowlist should allow any IP")
	}
	if !s.MatchIP("1.2.3.4", []string{"1.2.3.4"}) {
		t.Fatal("expected exact IP match")
	}
	if s.MatchIP("1.2.3.4", []string{"5.6.7.8"}) {
		t.Fatal("expected IP mismatch")
	}
}

func TestAPIKeyService_CheckIPRestriction(t *testing.T) {
	s := &APIKeyService{}
	key := &repository.APIKey{AllowedIPs: []string{"203.0.113.1"}}
	if !s.CheckIPRestriction("203.0.113.1", key) {
		t.Fatal("expected allowed IP")
	}
	if s.CheckIPRestriction("203.0.113.2", key) {
		t.Fatal("expected denied IP")
	}
	key.AllowedIPs = nil
	if !s.CheckIPRestriction("any", key) {
		t.Fatal("empty IP allowlist should allow any IP")
	}
}

func TestValidateScopes(t *testing.T) {
	if err := validateScopes(nil); err != ErrAPIKeyInvalidScope {
		t.Fatalf("empty scopes: want ErrAPIKeyInvalidScope got %v", err)
	}
	if err := validateScopes([]string{integration.ScopeStream}); err != nil {
		t.Fatalf("valid scope: %v", err)
	}
	if err := validateScopes([]string{"admin:all"}); err != ErrAPIKeyInvalidScope {
		t.Fatalf("unknown scope: want ErrAPIKeyInvalidScope got %v", err)
	}
}

func TestValidateAPIKeyStatus(t *testing.T) {
	if err := validateAPIKeyStatus("active"); err != nil {
		t.Fatal(err)
	}
	if err := validateAPIKeyStatus("revoked"); err != nil {
		t.Fatal(err)
	}
	if err := validateAPIKeyStatus("paused"); err != ErrAPIKeyInvalidStatus {
		t.Fatalf("want ErrAPIKeyInvalidStatus got %v", err)
	}
}
