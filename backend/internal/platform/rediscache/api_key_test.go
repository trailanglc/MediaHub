package rediscache

import (
	"testing"

	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/google/uuid"
)

func TestCachedAPIKeyRoundTrip(t *testing.T) {
	t.Parallel()
	root := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	createdBy := int64(42)
	src := &repository.APIKey{
		ID:                 7,
		PublicID:           uuid.New(),
		KeyHash:            "abc123",
		Scopes:             []string{"stream", "media:read"},
		AllowedIPs:         []string{"203.0.113.1"},
		RootFolderPublicID: &root,
		Status:             "active",
		CreatedBy:          &createdBy,
	}
	cached := CachedAPIKeyFromRecord(src)
	out := APIKeyFromCached(cached)
	if out.ID != src.ID || out.KeyHash != src.KeyHash || out.Status != "active" {
		t.Fatalf("round trip mismatch: %#v", out)
	}
	if len(out.Scopes) != 2 || out.CreatedBy == nil || *out.CreatedBy != 42 {
		t.Fatalf("scopes/created_by: %#v", out)
	}
}

func TestKeyAPIKeyHash(t *testing.T) {
	t.Parallel()
	if KeyAPIKeyHash("deadbeef") != PrefixAPIKeyHash+"deadbeef" {
		t.Fatal("unexpected cache key format")
	}
}
