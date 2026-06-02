package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/anhtuanlc/mediahub/internal/middleware"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ObjectHandler struct {
	objects *service.MediaObjectService
}

func NewObjectHandler(objects *service.MediaObjectService) *ObjectHandler {
	return &ObjectHandler{objects: objects}
}

func (h *ObjectHandler) List(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "authentication required"})
		return
	}

	var parentPID *uuid.UUID
	if raw := strings.TrimSpace(c.Query("parent_id")); raw != "" {
		pid, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid parent_id"})
			return
		}
		parentPID = &pid
	}

	var types []string
	if raw := strings.TrimSpace(c.Query("type")); raw != "" {
		types = strings.Split(raw, ",")
	}

	cursor, _ := strconv.ParseInt(c.DefaultQuery("cursor", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	items, next, err := h.objects.List(c.Request.Context(), u.ID, u.Role, service.ListObjectsInput{
		ParentPublicID: parentPID,
		Types:          types,
		Query:          c.Query("q"),
		Cursor:         cursor,
		Limit:          limit,
	})
	if err != nil {
		writeMediaError(c, err)
		return
	}
	resp := gin.H{"items": items}
	if next != nil {
		resp["next_cursor"] = *next
	}
	c.JSON(http.StatusOK, resp)
}

func (h *ObjectHandler) ListTrash(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	var types []string
	if raw := strings.TrimSpace(c.Query("type")); raw != "" {
		types = strings.Split(raw, ",")
	}
	cursor, _ := strconv.ParseInt(c.DefaultQuery("cursor", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	items, next, err := h.objects.ListTrash(c.Request.Context(), u.ID, u.Role, service.ListObjectsInput{
		Types:  types,
		Query:  c.Query("q"),
		Cursor: cursor,
		Limit:  limit,
	})
	if err != nil {
		writeMediaError(c, err)
		return
	}
	resp := gin.H{"items": items}
	if next != nil {
		resp["next_cursor"] = *next
	}
	c.JSON(http.StatusOK, resp)
}

func (h *ObjectHandler) Search(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	var types []string
	if raw := strings.TrimSpace(c.Query("type")); raw != "" {
		types = strings.Split(raw, ",")
	}
	cursor, _ := strconv.ParseInt(c.DefaultQuery("cursor", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	items, next, err := h.objects.Search(c.Request.Context(), u.ID, u.Role, service.ListObjectsInput{
		Types:  types,
		Query:  c.Query("q"),
		Cursor: cursor,
		Limit:  limit,
	})
	if err != nil {
		writeMediaError(c, err)
		return
	}
	resp := gin.H{"items": items}
	if next != nil {
		resp["next_cursor"] = *next
	}
	c.JSON(http.StatusOK, resp)
}

type batchPreviewRequest struct {
	IDs []string `json:"ids" binding:"required"`
}

func (h *ObjectHandler) BatchPreviewURLs(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	var req batchPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}
	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, raw := range req.IDs {
		pid, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid id in list"})
			return
		}
		ids = append(ids, pid)
	}
	urls, err := h.objects.BatchAccessURLs(c.Request.Context(), u.ID, u.Role, ids)
	if err != nil {
		writeMediaError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"urls": urls})
}

type bulkRenameRequest struct {
	ObjectIDs []string `json:"object_ids" binding:"required"`
	Mode      string   `json:"mode" binding:"required"`
	Value     string   `json:"value" binding:"required"`
}

func (h *ObjectHandler) BulkRename(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	var req bulkRenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}
	ids := make([]uuid.UUID, 0, len(req.ObjectIDs))
	for _, raw := range req.ObjectIDs {
		pid, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid object_ids"})
			return
		}
		ids = append(ids, pid)
	}
	out, err := h.objects.BulkRename(c.Request.Context(), u.ID, u.Role, service.BulkRenameInput{
		ObjectPublicIDs: ids,
		Mode:            req.Mode,
		Value:           req.Value,
	}, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeMediaError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *ObjectHandler) Get(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	publicID, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	obj, err := h.objects.Get(c.Request.Context(), u.ID, u.Role, publicID)
	if err != nil {
		writeMediaError(c, err)
		return
	}
	c.JSON(http.StatusOK, obj)
}

type createFolderRequest struct {
	ParentID string `json:"parent_id" binding:"required"`
	Name     string `json:"name" binding:"required"`
}

func (h *ObjectHandler) CreateFolder(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	var req createFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}
	parentPID, err := uuid.Parse(req.ParentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid parent_id"})
		return
	}
	obj, err := h.objects.CreateFolder(c.Request.Context(), u.ID, u.Role, service.CreateFolderInput{
		ParentPublicID: parentPID,
		Name:           req.Name,
	}, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeMediaError(c, err)
		return
	}
	c.JSON(http.StatusCreated, obj)
}

type patchObjectRequest struct {
	Name     *string `json:"name"`
	ParentID *string `json:"parent_id"`
}

func (h *ObjectHandler) Patch(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	publicID, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	var req patchObjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}
	var parentPID *uuid.UUID
	if req.ParentID != nil {
		pid, err := uuid.Parse(*req.ParentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid parent_id"})
			return
		}
		parentPID = &pid
	}
	obj, err := h.objects.Patch(c.Request.Context(), u.ID, u.Role, publicID, service.PatchObjectInput{
		Name:           req.Name,
		ParentPublicID: parentPID,
	}, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeMediaError(c, err)
		return
	}
	c.JSON(http.StatusOK, obj)
}

func (h *ObjectHandler) Delete(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	publicID, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	out, err := h.objects.Delete(c.Request.Context(), u.ID, u.Role, publicID, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeMediaError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *ObjectHandler) Restore(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	publicID, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	obj, err := h.objects.Restore(c.Request.Context(), u.ID, u.Role, publicID, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeMediaError(c, err)
		return
	}
	c.JSON(http.StatusOK, obj)
}

func (h *ObjectHandler) Purge(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	publicID, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	out, err := h.objects.Purge(c.Request.Context(), u.ID, u.Role, publicID, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeMediaError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *ObjectHandler) EmptyTrash(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	out, err := h.objects.EmptyTrash(c.Request.Context(), u.ID, u.Role, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeMediaError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func writeMediaError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrMediaAccessDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "insufficient permissions"})
	case errors.Is(err, repository.ErrMediaObjectNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "resource not found"})
	case errors.Is(err, service.ErrMediaInvalidName):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid name"})
	case errors.Is(err, service.ErrMediaInvalidParent):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid parent folder"})
	case errors.Is(err, service.ErrMediaCannotMoveRoot):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "cannot modify root folder"})
	case errors.Is(err, repository.ErrInvalidMove):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid move target"})
	case errors.Is(err, service.ErrMediaParentDeleted):
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "message": "parent folder is deleted; restore parent first"})
	case errors.Is(err, service.ErrMediaNotDeleted):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "object is not in trash"})
	case errors.Is(err, service.ErrMediaSearchQueryReq):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "search query q is required"})
	case errors.Is(err, service.ErrMediaBulkRenameLimit):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": fmt.Sprintf("maximum %d objects per bulk rename", service.MaxBulkRenameObjects)})
	case errors.Is(err, service.ErrMediaBulkRenameMode):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "mode must be prefix or suffix"})
	case errors.Is(err, service.ErrMediaFolderNotEmpty):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "folder is not empty"})
	default:
		if strings.Contains(err.Error(), "too many ids") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "operation failed"})
	}
}
