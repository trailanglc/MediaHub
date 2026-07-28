package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	CookieAccess  = "access_token"
	CookieRefresh = "refresh_token"
)

type CookieConfig struct {
	Secure bool
}

func SetAuthCookies(c *gin.Context, cfg CookieConfig, accessToken, refreshToken string, accessTTL, refreshTTL time.Duration) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieAccess,
		Value:    accessToken,
		Path:     "/",
		MaxAge:   int(accessTTL.Seconds()),
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteLaxMode,
	})
	// Path=/ so Next.js proxy can see the cookie on app navigations and
	// refresh an expired access token instead of bouncing to /login.
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieRefresh,
		Value:    refreshToken,
		Path:     "/",
		MaxAge:   int(refreshTTL.Seconds()),
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearAuthCookies(c *gin.Context, cfg CookieConfig) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieAccess,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieRefresh,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteLaxMode,
	})
	// Clear legacy Path=/api/auth refresh cookies from older builds.
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieRefresh,
		Value:    "",
		Path:     "/api/auth",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func AccessTokenFromRequest(c *gin.Context) string {
	if tok, err := c.Cookie(CookieAccess); err == nil && tok != "" {
		return tok
	}
	const prefix = "Bearer "
	h := c.GetHeader("Authorization")
	if len(h) > len(prefix) && h[:len(prefix)] == prefix {
		return h[len(prefix):]
	}
	return ""
}

func RefreshTokenFromRequest(c *gin.Context) string {
	if tok, err := c.Cookie(CookieRefresh); err == nil && tok != "" {
		return tok
	}
	return ""
}
