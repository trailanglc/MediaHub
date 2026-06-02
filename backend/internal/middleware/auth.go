package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/anhtuanlc/mediahub/internal/auth"
	"github.com/anhtuanlc/mediahub/internal/platform"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const AuthUserKey = "auth_user"

type AuthUser struct {
	ID       int64
	PublicID uuid.UUID
	Email    string
	Role     string
}

type AuthMiddleware struct {
	issuer      *auth.TokenIssuer
	revoke      *platform.TokenRevocation
	sessions    *platform.SessionInvalidation
	users       *repository.UserRepository
}

func NewAuthMiddleware(
	issuer *auth.TokenIssuer,
	revoke *platform.TokenRevocation,
	sessions *platform.SessionInvalidation,
	users *repository.UserRepository,
) *AuthMiddleware {
	return &AuthMiddleware{issuer: issuer, revoke: revoke, sessions: sessions, users: users}
}

func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := auth.AccessTokenFromRequest(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "authentication required",
			})
			return
		}
		claims, err := m.issuer.ParseAccess(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "invalid or expired token",
			})
			return
		}
		if claims.ID != "" {
			revoked, err := m.revoke.IsRevoked(c.Request.Context(), claims.ID)
			if err != nil || revoked {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error":   "unauthorized",
					"message": "token revoked",
				})
				return
			}
		}
		publicID, err := uuid.Parse(claims.Subject)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "invalid token subject",
			})
			return
		}
		u, err := m.users.GetByPublicID(c.Request.Context(), publicID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "user not found",
			})
			return
		}
		if u.Status != "active" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "account_disabled",
				"message": "account is disabled",
			})
			return
		}
		if m.sessions != nil && claims.IssuedAt != nil {
			valid, err := m.sessions.IssuedAtValid(c.Request.Context(), u.ID, claims.IssuedAt.Time)
			if err != nil || !valid {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error":   "unauthorized",
					"message": "session invalidated",
				})
				return
			}
		}
		c.Set(AuthUserKey, AuthUser{
			ID:       u.ID,
			PublicID: u.PublicID,
			Email:    u.Email,
			Role:     u.Role,
		})
		c.Set("access_jti", claims.ID)
		c.Next()
	}
}

func GetAuthUser(c *gin.Context) (AuthUser, bool) {
	v, ok := c.Get(AuthUserKey)
	if !ok {
		return AuthUser{}, false
	}
	u, ok := v.(AuthUser)
	return u, ok
}

func RequireOwner() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := GetAuthUser(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "authentication required",
			})
			return
		}
		if strings.ToLower(u.Role) != "owner" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "owner access required",
			})
			return
		}
		c.Next()
	}
}

func AccessJTITTL(claims *auth.AccessClaims) time.Duration {
	if claims.ExpiresAt == nil {
		return 15 * time.Minute
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl < 0 {
		return 0
	}
	return ttl
}
