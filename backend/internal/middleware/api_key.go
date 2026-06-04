package middleware

import (
	"net/http"
	"strings"

	"github.com/anhtuanlc/mediahub/internal/integration"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/gin-gonic/gin"
)

const APIKeyContextKey = "api_key"

// RequireAPIKey validates X-API-Key and ensures the key has the given scope.
func RequireAPIKey(keys *service.APIKeyService, scope string) gin.HandlerFunc {
	return requireAPIKey(keys, []string{scope})
}

// RequireAPIKeyAny validates X-API-Key and ensures the key has at least one scope.
func RequireAPIKeyAny(keys *service.APIKeyService, scopes ...string) gin.HandlerFunc {
	return requireAPIKey(keys, scopes)
}

func requireAPIKey(keys *service.APIKeyService, scopes []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if keys == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error":   "service_unavailable",
				"message": "api keys not configured",
			})
			return
		}
		plain := strings.TrimSpace(c.GetHeader("X-API-Key"))
		if plain == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "X-API-Key required",
			})
			return
		}
		key, err := keys.VerifyAPIKey(c.Request.Context(), plain)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "invalid api key",
			})
			return
		}
		if !integration.HasAnyScope(key.Scopes, scopes...) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "insufficient api key scope",
			})
			return
		}
		if !keys.CheckIPRestriction(c.ClientIP(), key) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "api key ip restriction",
			})
			return
		}
		if key.CreatedBy == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "api key missing owner context",
			})
			return
		}
		c.Set(APIKeyContextKey, key)
		c.Next()
	}
}

func GetAPIKey(c *gin.Context) (*repository.APIKey, bool) {
	v, ok := c.Get(APIKeyContextKey)
	if !ok {
		return nil, false
	}
	k, ok := v.(*repository.APIKey)
	return k, ok
}
