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

type WebhookHandler struct {
	webhooks *service.WebhookService
}

func NewWebhookHandler(webhooks *service.WebhookService) *WebhookHandler {
	return &WebhookHandler{webhooks: webhooks}
}

type createWebhookRequest struct {
	URL    string   `json:"url" binding:"required"`
	Events []string `json:"events" binding:"required"`
}

func (h *WebhookHandler) List(c *gin.Context) {
	list, err := h.webhooks.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}

func (h *WebhookHandler) Create(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	var req createWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}
	res, err := h.webhooks.Create(c.Request.Context(), u.ID, service.CreateWebhookInput{
		URL:    req.URL,
		Events: req.Events,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *WebhookHandler) Delete(c *gin.Context) {
	publicID, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	if err := h.webhooks.Delete(c.Request.Context(), publicID); err != nil {
		if errors.Is(err, repository.ErrWebhookNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
