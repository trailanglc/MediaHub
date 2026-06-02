package handler

import (
	"errors"
	"net/http"

	"github.com/anhtuanlc/mediahub/internal/auth"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/gin-gonic/gin"
)

type SetupHandler struct {
	setup    *service.SetupService
	password *PasswordTransport
}

func NewSetupHandler(setup *service.SetupService, password *PasswordTransport) *SetupHandler {
	return &SetupHandler{setup: setup, password: password}
}

type createOwnerRequest struct {
	Email             string `json:"email" binding:"required"`
	Password          string `json:"password"`
	EncryptedPassword string `json:"encrypted_password"`
	SetupToken        string `json:"setup_token"`
}

func (h *SetupHandler) Status(c *gin.Context) {
	status, err := h.setup.GetStatus(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "failed to check setup status",
		})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *SetupHandler) CreateOwner(c *gin.Context) {
	var req createOwnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": err.Error(),
		})
		return
	}

	token := req.SetupToken
	if token == "" {
		token = c.GetHeader("X-Setup-Token")
	}

	plain, err := h.password.Resolve(c, req.Password, req.EncryptedPassword)
	if err != nil {
		return
	}

	result, err := h.setup.CreateOwner(c.Request.Context(), service.CreateOwnerInput{
		Email:      req.Email,
		Password:   plain,
		SetupToken: token,
		IP:         c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
	})
	if err != nil {
		writeSetupError(c, err)
		return
	}

	c.JSON(http.StatusCreated, result)
}

func writeSetupError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrSetupCompleted):
		c.JSON(http.StatusConflict, gin.H{
			"error":   "setup_already_completed",
			"message": "owner account already exists",
		})
	case errors.Is(err, service.ErrInvalidSetupToken):
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "invalid_setup_token",
			"message": "setup token is required or invalid",
		})
	case errors.Is(err, service.ErrInvalidEmail):
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "invalid email address",
		})
	case errors.Is(err, auth.ErrPasswordTooShort):
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": err.Error(),
		})
	case errors.Is(err, auth.ErrPasswordTooWeak):
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "failed to create owner",
		})
	}
}
