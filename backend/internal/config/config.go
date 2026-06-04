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
	APIAddr       string

	DBDSN     string
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
	ConvertMaxConcurrent int
	ConvertMinConcurrent int
	ConvertJobTimeout    time.Duration
	StreamRateLimitPerMin int

	// RedisFailClosed: when true (production/staging), Redis errors deny login lockout checks and stream rate limits.
	RedisFailClosed bool

	// TrustedProxies: CIDRs or IPs for gin trusted reverse proxies (ClientIP, X-Forwarded-*).
	TrustedProxies []string

	// HealthMetricsCacheTTL: cache host + storage usage metrics for system health.
	HealthMetricsCacheTTL time.Duration

	UploadInitPerMinute      int
	MaxPendingUploadsPerUser int

	// Resource governor: adaptive limits from host/Redis idle %.
	ResourceGovernorEnabled     bool
	ResourceSampleInterval      time.Duration
	ResourceCPUReservePercent   int
	ResourceRAMMinIdlePercent   int
	ResourceRedisMaxUsedPercent int
	ResourceGovernorPublish     bool
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
		APIAddr:             getEnv("API_ADDR", ":8080"),
		DBDSN:               os.Getenv("DB_DSN"),
		RedisAddr:           getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPoolSize:        getEnvInt("REDIS_POOL_SIZE", 32),
		RedisMinIdleConns:    getEnvInt("REDIS_MIN_IDLE_CONNS", 8),
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
		ConvertMaxConcurrent:  getEnvInt("CONVERT_MAX_CONCURRENT", 2),
		ConvertMinConcurrent:  getEnvInt("CONVERT_MIN_CONCURRENT", 1),
		ConvertJobTimeout:     getEnvDuration("CONVERT_JOB_TIMEOUT", 2*time.Hour),
		StreamRateLimitPerMin: getEnvInt("STREAM_RATE_LIMIT_PER_MIN", 120),
		HealthMetricsCacheTTL: defaultHealthMetricsCacheTTL,
		UploadInitPerMinute:      getEnvInt("UPLOAD_INIT_PER_MINUTE", 60),
		MaxPendingUploadsPerUser: getEnvInt("UPLOAD_MAX_PENDING_PER_USER", 10),
		ResourceGovernorEnabled:     getEnvBool("RESOURCE_GOVERNOR_ENABLED", true),
		ResourceSampleInterval:      getEnvDuration("RESOURCE_SAMPLE_INTERVAL", 10*time.Second),
		ResourceCPUReservePercent:   getEnvInt("RESOURCE_CPU_RESERVE_PERCENT", 40),
		ResourceRAMMinIdlePercent:   getEnvInt("RESOURCE_RAM_MIN_IDLE_PERCENT", 15),
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

	return cfg, nil
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
