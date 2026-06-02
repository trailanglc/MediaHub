package handler

import (
	"errors"
	"net/http"

	"github.com/anhtuanlc/mediahub/internal/auth"
	"github.com/anhtuanlc/mediahub/internal/middleware"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	auth     *service.AuthService
	password *PasswordTransport
}

func NewAuthHandler(authSvc *service.AuthService, password *PasswordTransport) *AuthHandler {
	return &AuthHandler{auth: authSvc, password: password}
}

type loginRequest struct {
	Email             string `json:"email" binding:"required"`
	Password          string `json:"password"`
	EncryptedPassword string `json:"encrypted_password"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}
	plain, err := h.password.Resolve(c, req.Password, req.EncryptedPassword)
	if err != nil {
		return
	}
	user, tokens, err := h.auth.Login(c.Request.Context(), req.Email, plain, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeAuthError(c, err)
		return
	}
	auth.SetAuthCookies(c, auth.CookieConfig{Secure: h.auth.CookieSecure()},
		tokens.AccessToken, tokens.RefreshToken, h.auth.AccessTTL(), h.auth.RefreshTTL())
	c.JSON(http.StatusOK, gin.H{"user": user})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	plain := auth.RefreshTokenFromRequest(c)
	user, tokens, err := h.auth.Refresh(c.Request.Context(), plain, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeAuthError(c, err)
		return
	}
	auth.SetAuthCookies(c, auth.CookieConfig{Secure: h.auth.CookieSecure()},
		tokens.AccessToken, tokens.RefreshToken, h.auth.AccessTTL(), h.auth.RefreshTTL())
	c.JSON(http.StatusOK, gin.H{"user": user})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var userID *int64
	var accessJTI string
	if u, ok := middleware.GetAuthUser(c); ok {
		userID = &u.ID
		accessJTI = c.GetString("access_jti")
	}
	plain := auth.RefreshTokenFromRequest(c)
	_ = h.auth.Logout(c.Request.Context(), plain, accessJTI, h.auth.AccessTTL(), c.ClientIP(), c.Request.UserAgent(), userID)
	auth.ClearAuthCookies(c, auth.CookieConfig{Secure: h.auth.CookieSecure()})
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) Me(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "authentication required"})
		return
	}
	user, err := h.auth.Me(c.Request.Context(), u.ID)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

func writeAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_credentials", "message": "invalid email or password"})
	case errors.Is(err, service.ErrAccountDisabled):
		c.JSON(http.StatusForbidden, gin.H{"error": "account_disabled", "message": "account is disabled"})
	case errors.Is(err, service.ErrTooManyAttempts):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too_many_requests", "message": "too many login attempts, try again later"})
	case errors.Is(err, service.ErrInvalidRefresh):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_refresh", "message": "invalid or expired refresh token"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "authentication failed"})
	}
}
