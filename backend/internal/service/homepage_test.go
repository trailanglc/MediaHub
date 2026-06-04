package service

import "testing"

func TestHomepageSettings_validate(t *testing.T) {
	hp := DefaultHomepageSettings()
	if err := hp.validate(); err != nil {
		t.Fatalf("default should be valid: %v", err)
	}

	bad := hp
	bad.MetaTitle = ""
	if err := bad.validate(); err == nil {
		t.Fatal("expected error for empty meta title")
	}

	bad = hp
	bad.FaviconObjectID = "not-a-url"
	if err := bad.validate(); err == nil {
		t.Fatal("expected error for bad favicon object id")
	}

	bad = hp
	bad.FaviconObjectID = "00000000-0000-4000-8000-000000000099"
	if err := bad.validate(); err != nil {
		t.Fatalf("valid object id should pass: %v", err)
	}
}

func TestNormalizeKeywords(t *testing.T) {
	got := normalizeKeywords([]string{" SEO ", "seo", "Media"})
	if len(got) != 2 {
		t.Fatalf("expected 2 keywords, got %v", got)
	}
}
