package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Tunables kept in code (not .env) to avoid config sprawl.
const (
	defaultJWTAccessTTL          = 15 * time.Minute
	defaultJWTRefreshTTL         = 72 * time.Hour
	defaultHealthMetricsCacheTTL = 30 * time.Second
)

type Config struct {
	AppEnv        string
	AppURL        string
	APIPublicURL  string
	CDNPublicURL  string
	APIAddr       string

	DBDSN      string
	DBMaxConns int
	DBMinConns int
	RedisAddr string
	RedisPoolSize        int
	RedisMinIdleConns    int
	RedisReadTimeout     time.Duration
	RedisWriteTimeout    time.Duration

	Storage StorageConfig

	JWTSecret           string
	JWTAccessTTL        time.Duration
	JWTRefreshTTL       time.Duration
	LoginMaxAttempts       int
	LoginLockoutWindow     time.Duration
	LoginRSAPrivateKeyPEM  string
	RequireEncryptedPassword bool
	SetupToken          string
	FFmpegPath          string
	FFprobePath         string
	StreamSigningSecret string
	ConvertMaxConcurrent  int
	ConvertMinConcurrent  int
	ConvertQueueMaxDepth int
	ConvertJobTimeout    time.Duration
	StreamRateLimitPerMin int
	// StreamSegmentRateLimitPerMin caps origin segment fetches per IP/video per minute
	// (fails open: never blocks playback on Redis errors). 0 derives 10x the playlist limit.
	StreamSegmentRateLimitPerMin int
	// StreamSegmentURLTTL is the rolling window for shared, edge-cacheable signed segment URLs.
	StreamSegmentURLTTL time.Duration
	// AssetDeliveryURLTTL is the rolling window for signed image/thumbnail/embed delivery URLs.
	AssetDeliveryURLTTL time.Duration
	// PresignedPutURLTTL is the validity window for direct PUT upload URLs.
	PresignedPutURLTTL time.Duration
	// ImageTransformCacheTTL caches on-the-fly resized images in Redis.
	ImageTransformCacheTTL time.Duration
	// StreamInternalRedirectPrefix, when set, makes the API emit X-Accel-Redirect for segments
	// so an internal nginx location streams bytes directly from object storage (Go does no I/O).
	StreamInternalRedirectPrefix string

	// RedisFailClosed: when true (production/staging), Redis errors deny login lockout checks and stream rate limits.
	RedisFailClosed bool

	// TrustedProxies: CIDRs or IPs for gin trusted reverse proxies (ClientIP, X-Forwarded-*).
	TrustedProxies []string

	// HealthMetricsCacheTTL: cache host + storage usage metrics for system health.
	HealthMetricsCacheTTL time.Duration

	UploadInitPerMinute      int
	MaxPendingUploadsPerUser int
	APIKeyUploadInitPerMin   int
	APIKeyConvertPerHour     int

	// Resource governor: adaptive limits from host/Redis idle %.
	ResourceGovernorEnabled     bool
	ResourceSampleInterval      time.Duration
	ResourceCPUReservePercent   int
	ResourceRAMMinIdlePercent   int
	ResourceRedisMaxUsedPercent int
	ResourceGovernorPublish     bool

	autoscaleNotes []string
}

type StorageConfig struct {
	Driver       string
	Endpoint     string
	Region       string
	Bucket       string
	AccessKey    string
	SecretKey    string
	UseSSL       bool
	UsePathStyle bool
	QuotaBytes   int64
}

func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:              getEnv("APP_ENV", "development"),
		AppURL:              getEnv("APP_URL", "http://localhost:3000"),
		APIPublicURL:        getEnv("API_PUBLIC_URL", "http://localhost:8080"),
		CDNPublicURL:        strings.TrimRight(strings.TrimSpace(os.Getenv("CDN_PUBLIC_URL")), "/"),
		APIAddr:             getEnv("API_ADDR", ":8080"),
		DBDSN:               os.Getenv("DB_DSN"),
		RedisAddr:           getEnv("REDIS_ADDR", "localhost:6379"),
		DBMaxConns:           getEnvInt("DB_MAX_CONNS", 0),
		DBMinConns:           getEnvInt("DB_MIN_CONNS", 0),
		RedisPoolSize:        getEnvInt("REDIS_POOL_SIZE", 0),
		RedisMinIdleConns:    getEnvInt("REDIS_MIN_IDLE_CONNS", 0),
		RedisReadTimeout:     getEnvDuration("REDIS_READ_TIMEOUT", 3*time.Second),
		RedisWriteTimeout:    getEnvDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
		JWTSecret:           os.Getenv("JWT_SECRET"),
		JWTAccessTTL:        defaultJWTAccessTTL,
		JWTRefreshTTL:       getEnvDuration("JWT_REFRESH_TTL", defaultJWTRefreshTTL),
		LoginMaxAttempts:    getEnvInt("LOGIN_MAX_ATTEMPTS", 5),
		LoginLockoutWindow:       getEnvDuration("LOGIN_LOCKOUT_WINDOW", 15*time.Minute),
		LoginRSAPrivateKeyPEM:    os.Getenv("LOGIN_RSA_PRIVATE_KEY"),
		RequireEncryptedPassword: getEnvBool("REQUIRE_ENCRYPTED_PASSWORD", false),
		SetupToken:          os.Getenv("SETUP_TOKEN"),
		FFmpegPath:          getEnv("FFMPEG_PATH", "/usr/bin/ffmpeg"),
		FFprobePath:           getEnv("FFPROBE_PATH", "/usr/bin/ffprobe"),
		StreamSigningSecret:   os.Getenv("STREAM_SIGNING_SECRET"),
		ConvertMaxConcurrent:   getEnvInt("CONVERT_MAX_CONCURRENT", 0),
		ConvertMinConcurrent:   getEnvInt("CONVERT_MIN_CONCURRENT", 0),
		ConvertQueueMaxDepth:   getEnvInt("CONVERT_QUEUE_MAX_DEPTH", 0),
		ConvertJobTimeout:     getEnvDuration("CONVERT_JOB_TIMEOUT", 2*time.Hour),
		StreamRateLimitPerMin: getEnvInt("STREAM_RATE_LIMIT_PER_MIN", 120),
		StreamSegmentRateLimitPerMin: getEnvInt("STREAM_SEGMENT_RATE_LIMIT_PER_MIN", 0),
		StreamSegmentURLTTL:          getEnvDuration("STREAM_SEGMENT_URL_TTL", time.Hour),
		AssetDeliveryURLTTL:          getEnvDuration("ASSET_DELIVERY_URL_TTL", 24*time.Hour),
		PresignedPutURLTTL:           getEnvDuration("PRESIGNED_PUT_URL_TTL", time.Hour),
		ImageTransformCacheTTL:       getEnvDuration("IMAGE_TRANSFORM_CACHE_TTL", 24*time.Hour),
		StreamInternalRedirectPrefix: strings.TrimRight(os.Getenv("STREAM_INTERNAL_REDIRECT_PREFIX"), "/"),
		HealthMetricsCacheTTL: defaultHealthMetricsCacheTTL,
		UploadInitPerMinute:      getEnvInt("UPLOAD_INIT_PER_MINUTE", 0),
		MaxPendingUploadsPerUser: getEnvInt("UPLOAD_MAX_PENDING_PER_USER", 10),
		APIKeyUploadInitPerMin:   getEnvInt("API_KEY_UPLOAD_INIT_PER_MIN", 120),
		APIKeyConvertPerHour:     getEnvInt("API_KEY_CONVERT_PER_HOUR", 60),
		ResourceGovernorEnabled:     getEnvBool("RESOURCE_GOVERNOR_ENABLED", true),
		ResourceSampleInterval:      getEnvDuration("RESOURCE_SAMPLE_INTERVAL", 10*time.Second),
		ResourceCPUReservePercent:   getEnvInt("RESOURCE_CPU_RESERVE_PERCENT", 0),
		ResourceRAMMinIdlePercent:   getEnvInt("RESOURCE_RAM_MIN_IDLE_PERCENT", 0),
		ResourceRedisMaxUsedPercent: getEnvInt("RESOURCE_REDIS_MAX_USED_PERCENT", 85),
		ResourceGovernorPublish:     getEnvBool("RESOURCE_GOVERNOR_PUBLISH", false),
		Storage: StorageConfig{
			Driver:       getEnv("STORAGE_DRIVER", "s3"),
			Endpoint:     os.Getenv("STORAGE_ENDPOINT"),
			Region:       getEnv("STORAGE_REGION", "us-east-1"),
			Bucket:       os.Getenv("STORAGE_BUCKET"),
			AccessKey:    os.Getenv("STORAGE_ACCESS_KEY"),
			SecretKey:    os.Getenv("STORAGE_SECRET_KEY"),
			UseSSL:       getEnvBool("STORAGE_USE_SSL", false),
			UsePathStyle: getEnvBool("STORAGE_USE_PATH_STYLE", true),
			QuotaBytes:   0,
		},
	}

	if cfg.DBDSN == "" {
		return nil, fmt.Errorf("DB_DSN is required")
	}

	requireSecrets := cfg.AppEnv == "production" || cfg.AppEnv == "staging"
	if requireSecrets && cfg.SetupToken == "" {
		return nil, fmt.Errorf("SETUP_TOKEN is required when APP_ENV=%s", cfg.AppEnv)
	}
	if requireSecrets && cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required when APP_ENV=%s", cfg.AppEnv)
	}
	if cfg.AppEnv == "production" {
		cfg.RequireEncryptedPassword = true
	}
	if cfg.StreamSigningSecret == "" && cfg.AppEnv == "development" {
		cfg.StreamSigningSecret = "dev_stream_signing_secret"
	}
	if requireSecrets && cfg.StreamSigningSecret == "" {
		return nil, fmt.Errorf("STREAM_SIGNING_SECRET is required when APP_ENV=%s", cfg.AppEnv)
	}

	cfg.RedisFailClosed = requireSecrets
	cfg.TrustedProxies = parseTrustedProxies(os.Getenv("TRUSTED_PROXIES"))

	applyAutoscale(cfg)

	return cfg, nil
}

// DeliveryBaseURL returns the public origin used in signed asset/embed URLs.
// CDN_PUBLIC_URL overrides API_PUBLIC_URL when set (nginx/CDN in front of /assets and /embed).
func (c *Config) DeliveryBaseURL() string {
	if c == nil {
		return "http://localhost:8080"
	}
	if c.CDNPublicURL != "" {
		return c.CDNPublicURL
	}
	return strings.TrimRight(strings.TrimSpace(c.APIPublicURL), "/")
}

func parseTrustedProxies(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{"127.0.0.1", "::1"}
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
