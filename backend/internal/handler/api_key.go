package handler

import (
	"errors"
	"net/http"
	"strings"

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
	Name               string   `json:"name" binding:"required"`
	Scopes             []string `json:"scopes"`
	AllowedIPs         []string `json:"allowed_ips"`
	RootFolderPublicID *string  `json:"root_folder_public_id"`
}

func parseRootFolderPublicID(raw *string) (*uuid.UUID, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	pid, err := uuid.Parse(strings.TrimSpace(*raw))
	if err != nil {
		return nil, err
	}
	return &pid, nil
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
	rootFolder, err := parseRootFolderPublicID(req.RootFolderPublicID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid root_folder_public_id"})
		return
	}
	res, err := h.keys.Create(c.Request.Context(), u.ID, c.ClientIP(), c.Request.UserAgent(), service.CreateAPIKeyInput{
		Name:               req.Name,
		Scopes:             req.Scopes,
		AllowedIPs:         req.AllowedIPs,
		RootFolderPublicID: rootFolder,
	})
	if err != nil {
		if apiKeyValidationError(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	c.JSON(http.StatusCreated, res)
}

func apiKeyValidationError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, service.ErrAPIKeyInvalidScope):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid scope"})
		return true
	case errors.Is(err, service.ErrAPIKeyInvalidRootFolder):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid root folder"})
		return true
	default:
		return false
	}
}

type patchAPIKeyRequest struct {
	Name               *string   `json:"name"`
	Scopes             *[]string `json:"scopes"`
	AllowedIPs         []string  `json:"allowed_ips"`
	RootFolderPublicID *string  `json:"root_folder_public_id"`
	Status             *string  `json:"status"`
	ClearRootFolder    bool     `json:"clear_root_folder"`
}

func (h *APIKeyHandler) Patch(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
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
	var rootFolderPatch **uuid.UUID
	if req.ClearRootFolder {
		nilPID := (*uuid.UUID)(nil)
		rootFolderPatch = &nilPID
	} else if req.RootFolderPublicID != nil {
		pid, err := parseRootFolderPublicID(req.RootFolderPublicID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid root_folder_public_id"})
			return
		}
		rootFolderPatch = &pid
	}
	var scopes []string
	if req.Scopes != nil {
		scopes = *req.Scopes
	}
	dto, err := h.keys.Update(c.Request.Context(), u.ID, c.ClientIP(), c.Request.UserAgent(), publicID, req.Name, scopes, req.AllowedIPs, rootFolderPatch, req.Status)
	if err != nil {
		if errors.Is(err, repository.ErrAPIKeyNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		if errors.Is(err, service.ErrAPIKeyInvalidScope) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid scope"})
			return
		}
		if errors.Is(err, service.ErrAPIKeyInvalidStatus) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid status"})
			return
		}
		if errors.Is(err, service.ErrAPIKeyInvalidRootFolder) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid root folder"})
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
