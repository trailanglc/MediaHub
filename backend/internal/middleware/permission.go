package middleware

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/anhtuanlc/mediahub/internal/authz"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequirePermission checks resource-level RBAC for the authenticated user.
// resourcePublicIDParam is the Gin path param name holding the resource UUID (e.g. "public_id").
func RequirePermission(
	authzSvc *authz.Service,
	objects *repository.MediaObjectRepository,
	action authz.Action,
	resourcePublicIDParam string,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := GetAuthUser(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "authentication required",
			})
			return
		}
		if u.Role == "owner" {
			c.Next()
			return
		}

		raw := c.Param(resourcePublicIDParam)
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error":   "validation_error",
				"message": "resource id required",
			})
			return
		}
		publicID, err := uuid.Parse(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error":   "validation_error",
				"message": "invalid resource id",
			})
			return
		}

		obj, err := objects.GetByPublicID(c.Request.Context(), publicID)
		if err != nil {
			if errors.Is(err, repository.ErrMediaObjectNotFound) {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
					"error":   "not_found",
					"message": "resource not found",
				})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":   "internal_error",
				"message": "failed to resolve resource",
			})
			return
		}

		okPerm, err := authzSvc.HasPermission(c.Request.Context(), u.ID, u.Role, obj.ID, action)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":   "internal_error",
				"message": "permission check failed",
			})
			return
		}
		if !okPerm {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "insufficient permissions",
			})
			return
		}
		c.Set("resource_id", strconv.FormatInt(obj.ID, 10))
		c.Next()
	}
}
