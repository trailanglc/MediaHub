package service

import (
	"context"
	"testing"
	"time"

	"github.com/anhtuanlc/mediahub/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		AppEnv:              "development",
		AppURL:              "http://localhost:3000",
		DBDSN:               "postgres://x",
		RedisAddr:           "localhost:16379",
		LoginMaxAttempts:    5,
		LoginLockoutWindow:  15 * time.Minute,
		RequireEncryptedPassword: false,
		Storage: config.StorageConfig{
			Driver:   "s3",
			Endpoint: "http://localhost:9000",
			Bucket:   "mediahub",
		},
	}
}

func TestSettingsService_patchToUpserts_validation(t *testing.T) {
	s := &SettingsService{cfg: testConfig()}

	empty := ""
	_, err := s.patchToUpserts(context.Background(), SettingsPatch{
		Workspace: &SettingsWorkspacePatch{Name: &empty},
	})
	if err == nil {
		t.Fatal("expected error for empty workspace name")
	}

	badURL := "not-a-url"
	_, err = s.patchToUpserts(context.Background(), SettingsPatch{
		Workspace: &SettingsWorkspacePatch{PublicURL: &badURL},
	})
	if err == nil {
		t.Fatal("expected error for bad public url")
	}

	zero := int64(0)
	_, err = s.patchToUpserts(context.Background(), SettingsPatch{
		Media: &SettingsMediaPatch{MaxUploadBytes: &zero},
	})
	if err == nil {
		t.Fatal("expected error for zero max upload")
	}

	upserts, err := s.patchToUpserts(context.Background(), SettingsPatch{})
	if err != nil || len(upserts) != 0 {
		t.Fatalf("expected empty upserts, got %v err=%v", upserts, err)
	}

	name := "My Hub"
	attempts := 10
	upserts, err = s.patchToUpserts(context.Background(), SettingsPatch{
		Workspace: &SettingsWorkspacePatch{Name: &name},
		Security:  &SettingsSecurityPatch{LoginMaxAttempts: &attempts},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(upserts) != 2 {
		t.Fatalf("expected 2 upserts, got %d", len(upserts))
	}

	badConc := 0
	_, err = s.patchToUpserts(context.Background(), SettingsPatch{
		Download: &SettingsDownloadPatch{MaxConcurrent: &badConc},
	})
	if err == nil {
		t.Fatal("expected error for download.max_concurrent < 1")
	}
	okConc := 4
	okChunk := 3
	okJobTO := 3600
	okAnalyze := 45
	okDepth := 100
	upserts, err = s.patchToUpserts(context.Background(), SettingsPatch{
		Download: &SettingsDownloadPatch{
			MaxConcurrent:         &okConc,
			ChunkConcurrency:      &okChunk,
			JobTimeoutSeconds:     &okJobTO,
			AnalyzeTimeoutSeconds: &okAnalyze,
			QueueMaxDepth:         &okDepth,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(upserts) != 5 {
		t.Fatalf("expected 5 download upserts, got %d", len(upserts))
	}
}

func TestNormalizeDomains(t *testing.T) {
	got := normalizeDomains([]string{"  Example.COM ", "", "cdn.test"})
	if len(got) != 2 || got[0] != "example.com" || got[1] != "cdn.test" {
		t.Fatalf("unexpected: %v", got)
	}
}
