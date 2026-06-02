package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Tunables kept in code (not .env) to avoid config sprawl.
const (
	defaultJWTAccessTTL          = 15 * time.Minute
	defaultJWTRefreshTTL         = 72 * time.Hour
	defaultHealthMetricsCacheTTL = 30 * time.Second
)

type Config struct {
	AppEnv  string
	AppURL  string
	APIAddr string

	DBDSN     string
	RedisAddr string

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

	// HealthMetricsCacheTTL: cache host + storage usage metrics for system health.
	HealthMetricsCacheTTL time.Duration

	UploadInitPerMinute      int
	MaxPendingUploadsPerUser int
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
		APIAddr:             getEnv("API_ADDR", ":8080"),
		DBDSN:               os.Getenv("DB_DSN"),
		RedisAddr:           getEnv("REDIS_ADDR", "localhost:6379"),
		JWTSecret:           os.Getenv("JWT_SECRET"),
		JWTAccessTTL:        defaultJWTAccessTTL,
		JWTRefreshTTL:       getEnvDuration("JWT_REFRESH_TTL", defaultJWTRefreshTTL),
		LoginMaxAttempts:    getEnvInt("LOGIN_MAX_ATTEMPTS", 5),
		LoginLockoutWindow:       getEnvDuration("LOGIN_LOCKOUT_WINDOW", 15*time.Minute),
		LoginRSAPrivateKeyPEM:    os.Getenv("LOGIN_RSA_PRIVATE_KEY"),
		RequireEncryptedPassword: getEnvBool("REQUIRE_ENCRYPTED_PASSWORD", false),
		SetupToken:          os.Getenv("SETUP_TOKEN"),
		FFmpegPath:          getEnv("FFMPEG_PATH", "/usr/bin/ffmpeg"),
		FFprobePath:         getEnv("FFPROBE_PATH", "/usr/bin/ffprobe"),
		StreamSigningSecret:     os.Getenv("STREAM_SIGNING_SECRET"),
		HealthMetricsCacheTTL:    defaultHealthMetricsCacheTTL,
		UploadInitPerMinute:      getEnvInt("UPLOAD_INIT_PER_MINUTE", 60),
		MaxPendingUploadsPerUser: getEnvInt("UPLOAD_MAX_PENDING_PER_USER", 10),
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

	return cfg, nil
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
