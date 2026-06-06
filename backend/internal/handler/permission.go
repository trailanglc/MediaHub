package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/anhtuanlc/mediahub/internal/authz"
	"github.com/anhtuanlc/mediahub/internal/middleware"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PermissionHandler struct {
	perms   *service.PermissionService
	authz   *authz.Service
	objects *repository.MediaObjectRepository
}

func NewPermissionHandler(
	perms *service.PermissionService,
	authzSvc *authz.Service,
	objects *repository.MediaObjectRepository,
) *PermissionHandler {
	return &PermissionHandler{perms: perms, authz: authzSvc, objects: objects}
}

func (h *PermissionHandler) canDelegateOnResource(c *gin.Context, actor middleware.AuthUser, resourceID int64) bool {
	if authz.IsOwnerRole(actor.Role) {
		return true
	}
	if h.authz == nil {
		return false
	}
	for _, action := range []authz.Action{authz.ActionManage, authz.ActionShare} {
		ok, err := h.authz.HasPermission(c.Request.Context(), actor.ID, actor.Role, resourceID, action)
		if err == nil && ok {
			return true
		}
	}
	return false
}

func delegateMayGrant(permission string) bool {
	switch permission {
	case "manage", "share":
		return false
	default:
		return true
	}
}

func (h *PermissionHandler) ListMine(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "authentication required"})
		return
	}
	if strings.EqualFold(u.Role, "owner") {
		c.JSON(http.StatusOK, gin.H{"items": []gin.H{}})
		return
	}
	list, err := h.perms.ListMine(c.Request.Context(), u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "failed to list permissions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": permissionListResponse(list)})
}

func (h *PermissionHandler) List(c *gin.Context) {
	resourceID := c.Query("resource_id")
	if resourceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "resource_id is required"})
		return
	}
	rid, err := uuid.Parse(resourceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid resource_id"})
		return
	}
	actor, ok := middleware.GetAuthUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "authentication required"})
		return
	}
	obj, err := h.objects.GetByPublicID(c.Request.Context(), rid)
	if err != nil {
		if errors.Is(err, repository.ErrMediaObjectNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "resource not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "failed to resolve resource"})
		return
	}
	if !h.canDelegateOnResource(c, actor, obj.ID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "insufficient permissions"})
		return
	}
	list, err := h.perms.ListByResource(c.Request.Context(), rid)
	if err != nil {
		if errors.Is(err, repository.ErrMediaObjectNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "resource not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "failed to list permissions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": permissionListResponse(list)})
}

type grantPermissionRequest struct {
	UserPublicID     string `json:"user_public_id" binding:"required"`
	ResourcePublicID string `json:"resource_public_id" binding:"required"`
	Permission       string `json:"permission" binding:"required"`
}

func (h *PermissionHandler) Grant(c *gin.Context) {
	var req grantPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}
	userPID, err := uuid.Parse(req.UserPublicID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid user_public_id"})
		return
	}
	resPID, err := uuid.Parse(req.ResourcePublicID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid resource_public_id"})
		return
	}
	actor, ok := middleware.GetAuthUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "authentication required"})
		return
	}
	obj, err := h.objects.GetByPublicID(c.Request.Context(), resPID)
	if err != nil {
		if errors.Is(err, repository.ErrMediaObjectNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "resource not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "failed to resolve resource"})
		return
	}
	if !h.canDelegateOnResource(c, actor, obj.ID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "insufficient permissions"})
		return
	}
	if !authz.IsOwnerRole(actor.Role) && !delegateMayGrant(req.Permission) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "only owner may grant manage or share permissions",
		})
		return
	}
	p, err := h.perms.Grant(c.Request.Context(), service.GrantPermissionInput{
		UserPublicID:     userPID,
		ResourcePublicID: resPID,
		Permission:       req.Permission,
	}, actor.ID, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writePermissionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"permission": permissionResponse(p)})
}

func (h *PermissionHandler) Revoke(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid id"})
		return
	}
	actor, ok := middleware.GetAuthUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "authentication required"})
		return
	}
	perm, err := h.perms.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrPermissionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "permission not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "failed to load permission"})
		return
	}
	if !h.canDelegateOnResource(c, actor, perm.ResourceID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "insufficient permissions"})
		return
	}
	if err := h.perms.Revoke(c.Request.Context(), id, actor.ID, c.ClientIP(), c.Request.UserAgent()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "failed to revoke permission"})
		return
	}
	c.Status(http.StatusNoContent)
}

func permissionListResponse(list []repository.Permission) []gin.H {
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, permissionItemResponse(&list[i]))
	}
	return items
}

func permissionResponse(p *repository.Permission) gin.H {
	return permissionItemResponse(p)
}

func permissionItemResponse(p *repository.Permission) gin.H {
	return gin.H{
		"id":                 p.ID,
		"user_public_id":     p.UserPublicID.String(),
		"user_email":         p.UserEmail,
		"resource_public_id": p.ResourcePublic.String(),
		"resource_name":      p.ResourceName,
		"resource_type":      p.ResourceType,
		"permission":         p.Permission,
		"created_at":         p.CreatedAt,
	}
}

func writePermissionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidPermission):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid permission"})
	case errors.Is(err, repository.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "user not found"})
	case errors.Is(err, repository.ErrMediaObjectNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "resource not found"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
	}
}
