package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/anhtuanlc/mediahub/internal/auth"
	"github.com/anhtuanlc/mediahub/internal/middleware"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type MemberHandler struct {
	members  *service.MemberService
	password *PasswordTransport
}

func NewMemberHandler(members *service.MemberService, password *PasswordTransport) *MemberHandler {
	return &MemberHandler{members: members, password: password}
}

func (h *MemberHandler) List(c *gin.Context) {
	cursor, _ := strconv.ParseInt(c.DefaultQuery("cursor", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	users, err := h.members.List(c.Request.Context(), cursor, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "failed to list members"})
		return
	}
	items := make([]gin.H, 0, len(users))
	for i := range users {
		items = append(items, userResponse(&users[i]))
	}
	resp := gin.H{"items": items}
	if len(users) == limit {
		resp["next_cursor"] = users[len(users)-1].ID
	}
	c.JSON(http.StatusOK, resp)
}

func (h *MemberHandler) Get(c *gin.Context) {
	publicID, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	u, perms, err := h.members.Get(c.Request.Context(), publicID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "member not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "failed to get member"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user":        userResponse(u),
		"permissions": permissionListResponse(perms),
	})
}

type createMemberRequest struct {
	Email             string `json:"email" binding:"required"`
	Password          string `json:"password"`
	EncryptedPassword string `json:"encrypted_password"`
	Role              string `json:"role" binding:"required"`
}

func (h *MemberHandler) Create(c *gin.Context) {
	var req createMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}
	plain, err := h.password.Resolve(c, req.Password, req.EncryptedPassword)
	if err != nil {
		return
	}
	actor, _ := middleware.GetAuthUser(c)
	u, err := h.members.Create(c.Request.Context(), service.CreateMemberInput{
		Email: req.Email, Password: plain, Role: req.Role,
	}, actor.ID, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeMemberError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user": userResponse(u)})
}

type updateMemberRequest struct {
	Role              *string `json:"role"`
	Status            *string `json:"status"`
	Password          *string `json:"password"`
	EncryptedPassword *string `json:"encrypted_password"`
}

func (h *MemberHandler) Update(c *gin.Context) {
	publicID, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	var req updateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}
	var passwordPtr *string
	if req.EncryptedPassword != nil && *req.EncryptedPassword != "" {
		plain, err := h.password.Resolve(c, "", *req.EncryptedPassword)
		if err != nil {
			return
		}
		passwordPtr = &plain
	} else if req.Password != nil && *req.Password != "" {
		plain, err := h.password.Resolve(c, *req.Password, "")
		if err != nil {
			return
		}
		passwordPtr = &plain
	}
	actor, _ := middleware.GetAuthUser(c)
	u, err := h.members.Update(c.Request.Context(), publicID, service.UpdateMemberParams{
		Role: req.Role, Status: req.Status, Password: passwordPtr,
	}, actor.ID, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeMemberError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": userResponse(u)})
}

func (h *MemberHandler) Delete(c *gin.Context) {
	publicID, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	actor, _ := middleware.GetAuthUser(c)
	if err := h.members.Disable(c.Request.Context(), publicID, actor.ID, c.ClientIP(), c.Request.UserAgent()); err != nil {
		writeMemberError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *MemberHandler) Restore(c *gin.Context) {
	publicID, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	actor, _ := middleware.GetAuthUser(c)
	u, err := h.members.Restore(c.Request.Context(), publicID, actor.ID, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeMemberError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": userResponse(u)})
}

func (h *MemberHandler) Purge(c *gin.Context) {
	publicID, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	actor, _ := middleware.GetAuthUser(c)
	if err := h.members.Purge(c.Request.Context(), publicID, actor.ID, c.ClientIP(), c.Request.UserAgent()); err != nil {
		writeMemberError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func userResponse(u *repository.User) gin.H {
	return gin.H{
		"public_id":  u.PublicID.String(),
		"email":      u.Email,
		"role":       u.Role,
		"status":     u.Status,
		"created_at": u.CreatedAt,
		"updated_at": u.UpdatedAt,
	}
}

func writeMemberError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidMemberRole):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "role must be manager or viewer"})
	case errors.Is(err, service.ErrInvalidMemberStatus):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "status must be active or disabled"})
	case errors.Is(err, service.ErrNoMemberFields):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "no fields to update"})
	case errors.Is(err, service.ErrInvalidEmail):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid email address"})
	case errors.Is(err, repository.ErrEmailAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "message": "email already registered"})
	case errors.Is(err, service.ErrCannotDisableOwner), errors.Is(err, repository.ErrCannotModifyOwner):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "cannot modify owner account"})
	case errors.Is(err, auth.ErrPasswordTooShort), errors.Is(err, auth.ErrPasswordTooWeak):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
	case errors.Is(err, repository.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "member not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "operation failed"})
	}
}
