package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/anhtuanlc/mediahub/internal/auth"
	"github.com/anhtuanlc/mediahub/internal/authz"
	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/anhtuanlc/mediahub/internal/handler"
	"github.com/anhtuanlc/mediahub/internal/middleware"
	"github.com/anhtuanlc/mediahub/internal/observability"
	"github.com/anhtuanlc/mediahub/internal/platform/cache"
	"github.com/anhtuanlc/mediahub/internal/platform/postgres"
	"github.com/anhtuanlc/mediahub/internal/platform/rediscache"
	platredis "github.com/anhtuanlc/mediahub/internal/platform/redis"
	"github.com/anhtuanlc/mediahub/internal/platform/session"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/anhtuanlc/mediahub/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type securityEnv struct {
	t      *testing.T
	Router *gin.Engine
	Pool   *pgxpool.Pool
	Redis  *redis.Client
	Auth   *service.AuthService
	Issuer *auth.TokenIssuer
	Users  *repository.UserRepository
	Revoke *session.TokenRevocation
}

func newSecurityEnv(t *testing.T) *securityEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://mediahub:Anhtuanlc.12@localhost:15432/mediahub?sslmode=disable"
	}
	pool, err := postgres.NewPool(context.Background(), dsn, 0, 0)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:16379"
	}
	rdb := platredis.NewClient(redisAddr)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		pool.Close()
		t.Skipf("redis not available: %v", err)
	}

	cfg := &config.Config{
		AppEnv:                   "development",
		AppURL:                   "http://localhost:3000",
		JWTSecret:                "test-jwt-secret-for-security-suite",
		JWTAccessTTL:             15 * time.Minute,
		JWTRefreshTTL:            72 * time.Hour,
		LoginMaxAttempts:         5,
		LoginLockoutWindow:       15 * time.Minute,
		RequireEncryptedPassword: false,
		HealthMetricsCacheTTL:    30 * time.Second,
	}

	userRepo := repository.NewUserRepository(pool)
	refreshRepo := repository.NewRefreshTokenRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)
	permRepo := repository.NewPermissionRepository(pool)
	mediaRepo := repository.NewMediaObjectRepository(pool)
	settingsRepo := repository.NewSettingsRepository(pool)

	tokenRevoke := session.NewTokenRevocation(rdb)
	sessionInvalidate := session.NewSessionInvalidation(rdb, cfg.JWTRefreshTTL)
	loginLimiter := session.NewLoginRateLimiter(rdb, cfg.LoginMaxAttempts, cfg.LoginLockoutWindow, cfg.RedisFailClosed)
	redisCache := rediscache.NewStore(rdb)

	authSvc := service.NewAuthService(userRepo, refreshRepo, auditRepo, tokenRevoke, loginLimiter, cfg)
	memberSvc := service.NewMemberService(userRepo, permRepo, refreshRepo, auditRepo, sessionInvalidate, redisCache)
	permSvc := service.NewPermissionService(permRepo, userRepo, mediaRepo, auditRepo)
	authzSvc := authz.NewService(permRepo)
	settingsSvc := service.NewSettingsService(settingsRepo, auditRepo, cfg, redisCache)
	setupSvc := service.NewSetupService(userRepo, cfg)

	authUserCache := &rediscache.AuthUserCache{Store: redisCache, Users: userRepo}
	authMW := middleware.NewAuthMiddleware(authSvc.Issuer(), tokenRevoke, sessionInvalidate, userRepo, authUserCache)

	logger := zap.NewNop()
	store, _ := storage.NewS3Storage(context.Background(), cfg.Storage, cfg.HealthMetricsCacheTTL)

	passwordCipher, err := auth.NewPasswordCipher("")
	if err != nil {
		t.Fatalf("password cipher: %v", err)
	}
	passwordTransport := &handler.PasswordTransport{
		Cipher:           passwordCipher,
		RequireEncrypted: false,
	}

	health := &handler.HealthHandler{
		DB:              pool,
		Redis:           rdb,
		Storage:         store,
		MetricsCacheTTL: cfg.HealthMetricsCacheTTL,
		HostCache:       cache.NewTTLCache[*observability.HostStats](cfg.HealthMetricsCacheTTL),
	}

	router := handler.NewRouter(handler.RouterDeps{
		Logger:         logger,
		Health:         health,
		System:         handler.NewSystemHandler(nil),
		TrustedProxies: cfg.TrustedProxies,
		Setup:    handler.NewSetupHandler(setupSvc, passwordTransport),
		Auth:     handler.NewAuthHandler(authSvc, passwordTransport, logger),
		Audit:    handler.NewAuditHandler(auditRepo),
		Member:   handler.NewMemberHandler(memberSvc, passwordTransport),
		Perm:     handler.NewPermissionHandler(permSvc, authzSvc, mediaRepo),
		Settings: handler.NewSettingsHandler(settingsSvc, nil, logger),
		Password: passwordTransport,
		AuthMW:   authMW,
		AppURL:   cfg.AppURL,
		AppEnv:   cfg.AppEnv,
	})

	t.Cleanup(func() {
		rdb.Close()
		pool.Close()
	})

	return &securityEnv{
		t:      t,
		Router: router,
		Pool:   pool,
		Redis:  rdb,
		Auth:   authSvc,
		Issuer: authSvc.Issuer(),
		Users:  userRepo,
		Revoke: tokenRevoke,
	}
}

func (e *securityEnv) do(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)
	return rec
}

func (e *securityEnv) doJSON(method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return e.do(req)
}

func (e *securityEnv) login(email, password string) *http.Cookie {
	rec := e.doJSON(http.MethodPost, "/api/auth/login", map[string]string{
		"email":    email,
		"password": password,
	}, nil)
	if rec.Code != http.StatusOK {
		e.t.Skipf("login %s failed (%d): set TEST_OWNER_EMAIL and TEST_OWNER_PASSWORD to a valid owner", email, rec.Code)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieAccess {
			return c
		}
	}
	e.t.Fatal("no access_token cookie after login")
	return nil
}

func (e *securityEnv) refreshCookie(loginRec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == auth.CookieRefresh {
			return c
		}
	}
	e.t.Fatal("no refresh_token cookie after login")
	return nil
}

func (e *securityEnv) withAccessCookie(cookie *http.Cookie, method, path string, body any) *httptest.ResponseRecorder {
	headers := map[string]string{}
	if cookie != nil {
		headers["Cookie"] = cookie.Name + "=" + cookie.Value
	}
	return e.doJSON(method, path, body, headers)
}

func (e *securityEnv) withBearer(token, method, path string, body any) *httptest.ResponseRecorder {
	return e.doJSON(method, path, body, map[string]string{
		"Authorization": "Bearer " + token,
	})
}

func (e *securityEnv) ownerCredentials() (email, password string) {
	e.t.Helper()
	email = os.Getenv("TEST_OWNER_EMAIL")
	password = os.Getenv("TEST_OWNER_PASSWORD")
	if password == "" {
		e.t.Skip("set TEST_OWNER_PASSWORD to the active owner password (optional: TEST_OWNER_EMAIL)")
	}
	if email != "" {
		return email, password
	}
	err := e.Pool.QueryRow(context.Background(), `
		SELECT email FROM users WHERE role = 'owner' AND status = 'active' LIMIT 1
	`).Scan(&email)
	if err != nil {
		e.t.Skipf("no owner in database: %v", err)
	}
	return email, password
}
