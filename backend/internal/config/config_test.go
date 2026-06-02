package config_test

import (
	"testing"

	"github.com/anhtuanlc/mediahub/internal/config"
)

func TestLoad_RequiresSecretsInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DB_DSN", "postgres://localhost/mediahub")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("SETUP_TOKEN", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when JWT_SECRET and SETUP_TOKEN missing in production")
	}
}

func TestLoad_RequiresSecretsInStaging(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	t.Setenv("DB_DSN", "postgres://localhost/mediahub")
	t.Setenv("JWT_SECRET", "staging-secret")
	t.Setenv("SETUP_TOKEN", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when SETUP_TOKEN missing in staging")
	}
}

func TestLoad_DevelopmentAllowsEmptyJWTSecret(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DB_DSN", "postgres://localhost/mediahub")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("SETUP_TOKEN", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("development load: %v", err)
	}
	if cfg.JWTSecret != "" {
		t.Fatalf("expected empty JWT_SECRET in env, got %q", cfg.JWTSecret)
	}
}
