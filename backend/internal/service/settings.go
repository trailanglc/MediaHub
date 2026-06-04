package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/anhtuanlc/mediahub/internal/platform/rediscache"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/google/uuid"
)

const (
	KeyWorkspaceName                  = "workspace.name"
	KeyWorkspacePublicURL             = "workspace.public_url"
	KeyMediaDefaultRootFolderPublicID = "media.default_root_folder_public_id"
	KeyMediaMaxUploadBytes            = "media.max_upload_bytes"
	KeyMediaDeleteEmptyFoldersOnly    = "media.delete_empty_folders_only"
	KeySecurityLoginMaxAttempts       = "security.login_max_attempts"
	KeySecurityLoginLockoutMinutes    = "security.login_lockout_minutes"
	KeyStreamingDefaultTokenTTL       = "streaming.default_token_ttl_seconds"
	KeyStreamingGlobalAllowedDomains  = "streaming.global_allowed_domains"
	KeyStorageQuotaBytes              = "storage.quota_bytes"
	KeyMaintenanceAuditRetentionDays  = "maintenance.audit_retention_days"
	KeyMaintenanceTrashRetentionDays  = "maintenance.trash_retention_days"
	// DefaultTrashRetentionDays is how long soft-deleted objects stay before auto-purge (0 = disabled).
	DefaultTrashRetentionDays = 30
	// DefaultMaxUploadBytes is the default per-file upload limit (5 GiB).
	DefaultMaxUploadBytes int64 = 5 * 1024 * 1024 * 1024
)

var (
	ErrInvalidSettings       = errors.New("invalid settings")
	ErrNoSettingsToUpdate    = errors.New("no settings to update")
	ErrSettingsFieldLocked   = errors.New("setting field is locked")
)

type SettingsService struct {
	repo  *repository.SettingsRepository
	audit *repository.AuditRepository
	cfg   *config.Config
	cache *rediscache.Store
}

func NewSettingsService(repo *repository.SettingsRepository, audit *repository.AuditRepository, cfg *config.Config, cache *rediscache.Store) *SettingsService {
	return &SettingsService{repo: repo, audit: audit, cfg: cfg, cache: cache}
}

func (s *SettingsService) invalidateCache(ctx context.Context) {
	if s.cache == nil || !s.cache.Enabled() {
		return
	}
	_ = s.cache.Delete(ctx, rediscache.KeySettingsV1)
	rediscache.InvalidateAllStreamVideos(ctx, s.cache)
}

// GlobalAllowedDomains returns streaming global allowlist (uses settings cache).
func (s *SettingsService) GlobalAllowedDomains(ctx context.Context) ([]string, error) {
	resp, err := s.Get(ctx)
	if err != nil || resp == nil {
		return nil, err
	}
	return append([]string(nil), resp.Editable.Streaming.GlobalAllowedDomains...), nil
}

type SettingsWorkspace struct {
	Name      string `json:"name"`
	PublicURL string `json:"public_url"`
}

type SettingsMedia struct {
	DefaultRootFolderPublicID string `json:"default_root_folder_public_id"`
	MaxUploadBytes            int64  `json:"max_upload_bytes"`
	DeleteEmptyFoldersOnly    bool   `json:"delete_empty_folders_only"`
}

type SettingsSecurity struct {
	LoginMaxAttempts   int `json:"login_max_attempts"`
	LoginLockoutMinutes int `json:"login_lockout_minutes"`
}

type SettingsStreaming struct {
	DefaultTokenTTLSeconds int      `json:"default_token_ttl_seconds"`
	GlobalAllowedDomains   []string `json:"global_allowed_domains"`
}

type SettingsStorage struct {
	QuotaBytes int64 `json:"quota_bytes"`
}

type SettingsMaintenance struct {
	AuditRetentionDays int `json:"audit_retention_days"`
	TrashRetentionDays int `json:"trash_retention_days"`
}

type SettingsEditable struct {
	Workspace   SettingsWorkspace   `json:"workspace"`
	Media       SettingsMedia       `json:"media"`
	Security    SettingsSecurity    `json:"security"`
	Streaming   SettingsStreaming   `json:"streaming"`
	Storage     SettingsStorage     `json:"storage"`
	Maintenance SettingsMaintenance `json:"maintenance"`
}

type SettingsInfrastructure struct {
	AppEnv              string `json:"app_env"`
	DBConfigured        bool   `json:"db_configured"`
	RedisAddr           string `json:"redis_addr"`
	StorageDriver       string `json:"storage_driver"`
	StorageEndpoint     string `json:"storage_endpoint"`
	StorageBucket       string `json:"storage_bucket"`
	StorageRegion       string `json:"storage_region"`
	StorageUseSSL       bool   `json:"storage_use_ssl"`
	JWTConfigured       bool   `json:"jwt_configured"`
	SetupTokenConfigured bool  `json:"setup_token_configured"`
	FFmpegPath          string `json:"ffmpeg_path"`
	FFprobePath         string `json:"ffprobe_path"`
}

type SettingsRuntimeLocked struct {
	RequireEncryptedPassword bool `json:"require_encrypted_password"`
	MinPasswordLength        int  `json:"min_password_length"`
}

type SettingsReadonly struct {
	Infrastructure SettingsInfrastructure `json:"infrastructure"`
	RuntimeLocked  SettingsRuntimeLocked  `json:"runtime_locked"`
}

type SettingsResponse struct {
	Editable SettingsEditable `json:"editable"`
	Readonly SettingsReadonly `json:"readonly"`
}

type SettingsPatch struct {
	Workspace   *SettingsWorkspacePatch   `json:"workspace,omitempty"`
	Media       *SettingsMediaPatch       `json:"media,omitempty"`
	Security    *SettingsSecurityPatch    `json:"security,omitempty"`
	Streaming   *SettingsStreamingPatch   `json:"streaming,omitempty"`
	Storage     *SettingsStoragePatch     `json:"storage,omitempty"`
	Maintenance *SettingsMaintenancePatch `json:"maintenance,omitempty"`
}

type SettingsWorkspacePatch struct {
	Name      *string `json:"name,omitempty"`
	PublicURL *string `json:"public_url,omitempty"`
}

type SettingsMediaPatch struct {
	DefaultRootFolderPublicID *string `json:"default_root_folder_public_id,omitempty"`
	MaxUploadBytes            *int64  `json:"max_upload_bytes,omitempty"`
	DeleteEmptyFoldersOnly    *bool   `json:"delete_empty_folders_only,omitempty"`
}

type SettingsSecurityPatch struct {
	LoginMaxAttempts    *int `json:"login_max_attempts,omitempty"`
	LoginLockoutMinutes *int `json:"login_lockout_minutes,omitempty"`
}

type SettingsStreamingPatch struct {
	DefaultTokenTTLSeconds *int      `json:"default_token_ttl_seconds,omitempty"`
	GlobalAllowedDomains   *[]string `json:"global_allowed_domains,omitempty"`
}

type SettingsStoragePatch struct {
	QuotaBytes *int64 `json:"quota_bytes,omitempty"`
}

type SettingsMaintenancePatch struct {
	AuditRetentionDays *int `json:"audit_retention_days,omitempty"`
	TrashRetentionDays *int `json:"trash_retention_days,omitempty"`
}

func (s *SettingsService) Get(ctx context.Context) (*SettingsResponse, error) {
	if s.cache != nil && s.cache.Enabled() {
		var cached SettingsResponse
		if hit, err := s.cache.GetJSON(ctx, rediscache.KeySettingsV1, &cached); err != nil {
			return nil, err
		} else if hit {
			return &cached, nil
		}
	}
	resp, err := s.loadSettings(ctx)
	if err != nil {
		return nil, err
	}
	if s.cache != nil && s.cache.Enabled() {
		_ = s.cache.SetJSON(ctx, rediscache.KeySettingsV1, resp, rediscache.TTLSettings)
	}
	return resp, nil
}

func (s *SettingsService) loadSettings(ctx context.Context) (*SettingsResponse, error) {
	raw, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	editable := s.buildEditable(raw)
	return &SettingsResponse{
		Editable: editable,
		Readonly: s.buildReadonly(),
	}, nil
}

func (s *SettingsService) Update(ctx context.Context, patch SettingsPatch, actorID int64, ip, userAgent string) (*SettingsResponse, error) {
	upserts, err := s.patchToUpserts(patch)
	if err != nil {
		return nil, err
	}
	if len(upserts) == 0 {
		return nil, ErrNoSettingsToUpdate
	}
	if err := s.repo.UpsertMany(ctx, upserts, actorID); err != nil {
		return nil, err
	}
	meta := make(map[string]string, len(upserts))
	for _, u := range upserts {
		meta[u.Key] = "updated"
	}
	_ = s.audit.Log(ctx, &actorID, "settings.update", "system_settings", nil, ip, userAgent, meta)
	s.invalidateCache(ctx)
	return s.Get(ctx)
}

func (s *SettingsService) buildEditable(raw map[string]json.RawMessage) SettingsEditable {
	lockoutMin := int(s.cfg.LoginLockoutWindow / time.Minute)
	if lockoutMin < 1 {
		lockoutMin = 15
	}
	quota := s.cfg.Storage.QuotaBytes
	return SettingsEditable{
		Workspace: SettingsWorkspace{
			Name:      stringVal(raw, KeyWorkspaceName, "MediaHub"),
			PublicURL: stringVal(raw, KeyWorkspacePublicURL, s.cfg.AppURL),
		},
		Media: SettingsMedia{
			DefaultRootFolderPublicID: stringVal(raw, KeyMediaDefaultRootFolderPublicID, repository.DefaultRootFolderPublicID),
			MaxUploadBytes:            int64Val(raw, KeyMediaMaxUploadBytes, DefaultMaxUploadBytes),
			DeleteEmptyFoldersOnly:    boolVal(raw, KeyMediaDeleteEmptyFoldersOnly, false),
		},
		Security: SettingsSecurity{
			LoginMaxAttempts:    intVal(raw, KeySecurityLoginMaxAttempts, s.cfg.LoginMaxAttempts),
			LoginLockoutMinutes: intVal(raw, KeySecurityLoginLockoutMinutes, lockoutMin),
		},
		Streaming: SettingsStreaming{
			DefaultTokenTTLSeconds: intVal(raw, KeyStreamingDefaultTokenTTL, 3600),
			GlobalAllowedDomains:   stringSliceVal(raw, KeyStreamingGlobalAllowedDomains, nil),
		},
		Storage: SettingsStorage{
			QuotaBytes: int64Val(raw, KeyStorageQuotaBytes, quota),
		},
		Maintenance: SettingsMaintenance{
			AuditRetentionDays: intVal(raw, KeyMaintenanceAuditRetentionDays, 90),
			TrashRetentionDays: intVal(raw, KeyMaintenanceTrashRetentionDays, DefaultTrashRetentionDays),
		},
	}
}

func (s *SettingsService) buildReadonly() SettingsReadonly {
	return SettingsReadonly{
		Infrastructure: SettingsInfrastructure{
			AppEnv:               s.cfg.AppEnv,
			DBConfigured:         s.cfg.DBDSN != "",
			RedisAddr:            s.cfg.RedisAddr,
			StorageDriver:        s.cfg.Storage.Driver,
			StorageEndpoint:      maskURL(s.cfg.Storage.Endpoint),
			StorageBucket:        s.cfg.Storage.Bucket,
			StorageRegion:        s.cfg.Storage.Region,
			StorageUseSSL:        s.cfg.Storage.UseSSL,
			JWTConfigured:        s.cfg.JWTSecret != "",
			SetupTokenConfigured: s.cfg.SetupToken != "",
			FFmpegPath:           s.cfg.FFmpegPath,
			FFprobePath:          s.cfg.FFprobePath,
		},
		RuntimeLocked: SettingsRuntimeLocked{
			RequireEncryptedPassword: s.cfg.RequireEncryptedPassword,
			MinPasswordLength:        8,
		},
	}
}

func (s *SettingsService) patchToUpserts(patch SettingsPatch) ([]repository.SettingUpsert, error) {
	var upserts []repository.SettingUpsert

	if patch.Workspace != nil {
		if patch.Workspace.Name != nil {
			name := strings.TrimSpace(*patch.Workspace.Name)
			if name == "" || len(name) > 100 {
				return nil, fmt.Errorf("%w: workspace.name", ErrInvalidSettings)
			}
			upserts = append(upserts, repository.SettingUpsert{Key: KeyWorkspaceName, Value: name})
		}
		if patch.Workspace.PublicURL != nil {
			u := strings.TrimSpace(*patch.Workspace.PublicURL)
			if _, err := url.ParseRequestURI(u); err != nil {
				return nil, fmt.Errorf("%w: workspace.public_url", ErrInvalidSettings)
			}
			upserts = append(upserts, repository.SettingUpsert{Key: KeyWorkspacePublicURL, Value: u})
		}
	}

	if patch.Media != nil {
		if patch.Media.DefaultRootFolderPublicID != nil {
			id := strings.TrimSpace(*patch.Media.DefaultRootFolderPublicID)
			if _, err := uuid.Parse(id); err != nil {
				return nil, fmt.Errorf("%w: media.default_root_folder_public_id", ErrInvalidSettings)
			}
			upserts = append(upserts, repository.SettingUpsert{Key: KeyMediaDefaultRootFolderPublicID, Value: id})
		}
		if patch.Media.MaxUploadBytes != nil {
			if *patch.Media.MaxUploadBytes < 1 {
				return nil, fmt.Errorf("%w: media.max_upload_bytes", ErrInvalidSettings)
			}
			upserts = append(upserts, repository.SettingUpsert{Key: KeyMediaMaxUploadBytes, Value: *patch.Media.MaxUploadBytes})
		}
		if patch.Media.DeleteEmptyFoldersOnly != nil {
			upserts = append(upserts, repository.SettingUpsert{Key: KeyMediaDeleteEmptyFoldersOnly, Value: *patch.Media.DeleteEmptyFoldersOnly})
		}
	}

	if patch.Security != nil {
		if patch.Security.LoginMaxAttempts != nil {
			if *patch.Security.LoginMaxAttempts < 1 || *patch.Security.LoginMaxAttempts > 100 {
				return nil, fmt.Errorf("%w: security.login_max_attempts", ErrInvalidSettings)
			}
			upserts = append(upserts, repository.SettingUpsert{Key: KeySecurityLoginMaxAttempts, Value: *patch.Security.LoginMaxAttempts})
		}
		if patch.Security.LoginLockoutMinutes != nil {
			if *patch.Security.LoginLockoutMinutes < 1 || *patch.Security.LoginLockoutMinutes > 1440 {
				return nil, fmt.Errorf("%w: security.login_lockout_minutes", ErrInvalidSettings)
			}
			upserts = append(upserts, repository.SettingUpsert{Key: KeySecurityLoginLockoutMinutes, Value: *patch.Security.LoginLockoutMinutes})
		}
	}

	if patch.Streaming != nil {
		if patch.Streaming.DefaultTokenTTLSeconds != nil {
			if *patch.Streaming.DefaultTokenTTLSeconds < 60 || *patch.Streaming.DefaultTokenTTLSeconds > 86400 {
				return nil, fmt.Errorf("%w: streaming.default_token_ttl_seconds", ErrInvalidSettings)
			}
			upserts = append(upserts, repository.SettingUpsert{Key: KeyStreamingDefaultTokenTTL, Value: *patch.Streaming.DefaultTokenTTLSeconds})
		}
		if patch.Streaming.GlobalAllowedDomains != nil {
			domains := normalizeDomains(*patch.Streaming.GlobalAllowedDomains)
			upserts = append(upserts, repository.SettingUpsert{Key: KeyStreamingGlobalAllowedDomains, Value: domains})
		}
	}

	if patch.Storage != nil && patch.Storage.QuotaBytes != nil {
		if *patch.Storage.QuotaBytes < 0 {
			return nil, fmt.Errorf("%w: storage.quota_bytes", ErrInvalidSettings)
		}
		upserts = append(upserts, repository.SettingUpsert{Key: KeyStorageQuotaBytes, Value: *patch.Storage.QuotaBytes})
	}

	if patch.Maintenance != nil {
		if patch.Maintenance.AuditRetentionDays != nil {
			if *patch.Maintenance.AuditRetentionDays < 1 || *patch.Maintenance.AuditRetentionDays > 3650 {
				return nil, fmt.Errorf("%w: maintenance.audit_retention_days", ErrInvalidSettings)
			}
			upserts = append(upserts, repository.SettingUpsert{Key: KeyMaintenanceAuditRetentionDays, Value: *patch.Maintenance.AuditRetentionDays})
		}
		if patch.Maintenance.TrashRetentionDays != nil {
			if *patch.Maintenance.TrashRetentionDays < 0 || *patch.Maintenance.TrashRetentionDays > 3650 {
				return nil, fmt.Errorf("%w: maintenance.trash_retention_days", ErrInvalidSettings)
			}
			upserts = append(upserts, repository.SettingUpsert{Key: KeyMaintenanceTrashRetentionDays, Value: *patch.Maintenance.TrashRetentionDays})
		}
	}

	return upserts, nil
}

// PurgeExpiredAuditLogs deletes audit_logs older than maintenance.audit_retention_days (0 = disabled).
func (s *SettingsService) PurgeExpiredAuditLogs(ctx context.Context) (int, error) {
	if s.audit == nil {
		return 0, nil
	}
	cutoff, days, err := s.maintenanceCutoff(ctx)
	if err != nil {
		return 0, err
	}
	if days <= 0 {
		return 0, nil
	}
	n, err := s.audit.DeleteOlderThan(ctx, cutoff, 1000)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// PurgeOldJobRecords deletes terminal convert/storage-deletion jobs using audit retention days.
func (s *SettingsService) PurgeOldJobRecords(ctx context.Context, videos *repository.VideoRepository, deletions *repository.StorageDeletionRepository) (int, error) {
	cutoff, days, err := s.maintenanceCutoff(ctx)
	if err != nil {
		return 0, err
	}
	if days <= 0 {
		return 0, nil
	}
	total := int64(0)
	if videos != nil {
		n, err := videos.DeleteFinishedJobsOlderThan(ctx, cutoff, 1000)
		if err != nil {
			return 0, err
		}
		total += n
	}
	if deletions != nil {
		n, err := deletions.DeleteTerminalOlderThan(ctx, cutoff, 1000)
		if err != nil {
			return 0, err
		}
		total += n
	}
	return int(total), nil
}

func (s *SettingsService) maintenanceCutoff(ctx context.Context) (time.Time, int, error) {
	settings, err := s.Get(ctx)
	if err != nil {
		return time.Time{}, 0, err
	}
	days := settings.Editable.Maintenance.AuditRetentionDays
	if days <= 0 {
		return time.Time{}, 0, nil
	}
	return time.Now().Add(-time.Duration(days) * 24 * time.Hour), days, nil
}

// PurgeStaleRefreshTokens deletes expired/revoked refresh tokens using audit retention days.
func (s *SettingsService) PurgeStaleRefreshTokens(ctx context.Context, refresh *repository.RefreshTokenRepository) (int, error) {
	if refresh == nil {
		return 0, nil
	}
	cutoff, days, err := s.maintenanceCutoff(ctx)
	if err != nil {
		return 0, err
	}
	if days <= 0 {
		return 0, nil
	}
	n, err := refresh.DeleteStale(ctx, cutoff, 1000)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// DeleteEmptyFoldersOnly returns whether folder delete requires an empty folder.
func (s *SettingsService) DeleteEmptyFoldersOnly(ctx context.Context) bool {
	raw, err := s.repo.GetAll(ctx)
	if err != nil {
		return false
	}
	return boolVal(raw, KeyMediaDeleteEmptyFoldersOnly, false)
}

func normalizeDomains(domains []string) []string {
	out := make([]string, 0, len(domains))
	for _, d := range domains {
		d = strings.TrimSpace(strings.ToLower(d))
		if d == "" {
			continue
		}
		out = append(out, d)
	}
	return out
}

func stringVal(raw map[string]json.RawMessage, key, fallback string) string {
	if v, ok := raw[key]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil && s != "" {
			return s
		}
	}
	return fallback
}

func intVal(raw map[string]json.RawMessage, key string, fallback int) int {
	if v, ok := raw[key]; ok {
		var n int
		if json.Unmarshal(v, &n) == nil {
			return n
		}
	}
	return fallback
}

func int64Val(raw map[string]json.RawMessage, key string, fallback int64) int64 {
	if v, ok := raw[key]; ok {
		var n int64
		if json.Unmarshal(v, &n) == nil {
			return n
		}
	}
	return fallback
}

func boolVal(raw map[string]json.RawMessage, key string, fallback bool) bool {
	if v, ok := raw[key]; ok {
		var b bool
		if json.Unmarshal(v, &b) == nil {
			return b
		}
	}
	return fallback
}

func stringSliceVal(raw map[string]json.RawMessage, key string, fallback []string) []string {
	if v, ok := raw[key]; ok {
		var sl []string
		if json.Unmarshal(v, &sl) == nil {
			return sl
		}
	}
	if fallback == nil {
		return []string{}
	}
	return fallback
}

func maskURL(endpoint string) string {
	if endpoint == "" {
		return ""
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "***"
	}
	if u.User != nil {
		u.User = url.UserPassword("***", "***")
	}
	return u.String()
}
