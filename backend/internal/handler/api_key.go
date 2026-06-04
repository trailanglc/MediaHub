package handler

import (
	"errors"
	"net/http"

	"github.com/anhtuanlc/mediahub/internal/middleware"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type APIKeyHandler struct {
	keys *service.APIKeyService
}

func NewAPIKeyHandler(keys *service.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{keys: keys}
}

func (h *APIKeyHandler) List(c *gin.Context) {
	list, err := h.keys.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}

type createAPIKeyRequest struct {
	Name           string   `json:"name" binding:"required"`
	Scopes         []string `json:"scopes"`
	AllowedDomains []string `json:"allowed_domains"`
	AllowedIPs     []string `json:"allowed_ips"`
}

func (h *APIKeyHandler) Create(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	var req createAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid body"})
		return
	}
	res, err := h.keys.Create(c.Request.Context(), u.ID, c.ClientIP(), c.Request.UserAgent(), service.CreateAPIKeyInput{
		Name:           req.Name,
		Scopes:         req.Scopes,
		AllowedDomains: req.AllowedDomains,
		AllowedIPs:     req.AllowedIPs,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	c.JSON(http.StatusCreated, res)
}

type patchAPIKeyRequest struct {
	Name           *string  `json:"name"`
	Scopes         []string `json:"scopes"`
	AllowedDomains []string `json:"allowed_domains"`
	AllowedIPs     []string `json:"allowed_ips"`
	Status         *string  `json:"status"`
}

func (h *APIKeyHandler) Patch(c *gin.Context) {
	publicID, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	var req patchAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid body"})
		return
	}
	dto, err := h.keys.Update(c.Request.Context(), publicID, req.Name, req.Scopes, req.AllowedDomains, req.AllowedIPs, req.Status)
	if err != nil {
		if errors.Is(err, repository.ErrAPIKeyNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	c.JSON(http.StatusOK, dto)
}

func (h *APIKeyHandler) Delete(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	publicID, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	if err := h.keys.Revoke(c.Request.Context(), u.ID, c.ClientIP(), c.Request.UserAgent(), publicID); err != nil {
		if errors.Is(err, repository.ErrAPIKeyNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
