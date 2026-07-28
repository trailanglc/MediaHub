package handler

import (
	"errors"
	"net/http"

	"github.com/anhtuanlc/mediahub/internal/middleware"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type VideoCategoryHandler struct {
	categories *service.VideoCategoryService
}

func NewVideoCategoryHandler(categories *service.VideoCategoryService) *VideoCategoryHandler {
	return &VideoCategoryHandler{categories: categories}
}

func (h *VideoCategoryHandler) List(c *gin.Context) {
	if _, ok := middleware.GetAuthUser(c); !ok {
		return
	}
	items, err := h.categories.List(c.Request.Context())
	if err != nil {
		writeVideoCategoryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *VideoCategoryHandler) Create(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	var req service.CreateVideoCategoryInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid body"})
		return
	}
	dto, err := h.categories.Create(c.Request.Context(), u.ID, req)
	if err != nil {
		writeVideoCategoryError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto)
}

func (h *VideoCategoryHandler) Patch(c *gin.Context) {
	if _, ok := middleware.GetAuthUser(c); !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	var req service.UpdateVideoCategoryInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid body"})
		return
	}
	dto, err := h.categories.Update(c.Request.Context(), pid, req)
	if err != nil {
		writeVideoCategoryError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto)
}

func (h *VideoCategoryHandler) Delete(c *gin.Context) {
	if _, ok := middleware.GetAuthUser(c); !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	if err := h.categories.Delete(c.Request.Context(), pid); err != nil {
		writeVideoCategoryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func writeVideoCategoryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrVideoCategoryNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "category not found"})
	case errors.Is(err, service.ErrVideoCategoryExists):
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "message": "category name already exists"})
	case errors.Is(err, service.ErrVideoCategoryInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "operation failed"})
	}
}
