package handler

import (
	"io"
	"net/http"
	"strconv"

	"github.com/anhtuanlc/mediahub/internal/integration"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/anhtuanlc/mediahub/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AssetDeliveryHandler struct {
	Objects   *repository.MediaObjectRepository
	Videos    *service.VideoService
	Delivery  *service.DeliveryService
	Transform *service.ImageTransformService
	Store     storage.ObjectStorage
}

func NewAssetDeliveryHandler(
	objects *repository.MediaObjectRepository,
	videos *service.VideoService,
	delivery *service.DeliveryService,
	transform *service.ImageTransformService,
	store storage.ObjectStorage,
) *AssetDeliveryHandler {
	return &AssetDeliveryHandler{
		Objects:   objects,
		Videos:    videos,
		Delivery:  delivery,
		Transform: transform,
		Store:     store,
	}
}

func (h *AssetDeliveryHandler) Serve(c *gin.Context) {
	if h.Delivery == nil || h.Store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}
	objectPID, err := uuid.Parse(c.Param("object_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid object id"})
		return
	}
	variant := c.Param("variant")
	exp, _ := strconv.ParseInt(c.Query("e"), 10, 64)
	sig := c.Query("s")
	if exp <= 0 || sig == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "missing signature"})
		return
	}
	if err := h.Delivery.VerifyAsset(objectPID.String(), variant, exp, sig); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "invalid or expired signature"})
		return
	}
	if variant == integration.AssetVariantEmbed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "use /embed for video embed"})
		return
	}

	m, err := h.Objects.GetByPublicID(c.Request.Context(), objectPID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	if variant == integration.AssetVariantFile {
		allowed, err := h.Videos.AllowSourceDownload(c.Request.Context(), m)
		if err != nil || !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "source download not allowed"})
			return
		}
	}

	tw, _ := strconv.Atoi(c.Query("w"))
	th, _ := strconv.Atoi(c.Query("h"))
	tfmt := c.Query("fmt")
	transformReq := service.ImageTransformRequest{
		ObjectID: objectPID.String(),
		Variant:  variant,
		Width:    tw,
		Height:   th,
		Format:   tfmt,
	}
	if h.Transform != nil && transformReq.HasTransform() && h.Transform.TransformableVariant(variant) {
		out, ct, err := h.Transform.Render(c.Request.Context(), m, transformReq)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
			return
		}
		setAssetPublicCORS(c)
		c.Header("Cache-Control", "public, max-age=86400")
		c.Header("Content-Type", ct)
		c.Header("Content-Length", strconv.Itoa(len(out)))
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write(out)
		return
	}

	key, ct, err := service.StorageKeyForVariant(m, variant)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "variant unavailable"})
		return
	}

	setAssetPublicCORS(c)
	rangeHeader := c.GetHeader("Range")
	body, info, err := h.Store.GetRange(c.Request.Context(), key, rangeHeader)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	defer body.Close()

	if info.ContentType != "" {
		ct = info.ContentType
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Type", ct)
	if info.ETag != "" {
		c.Header("ETag", `"`+info.ETag+`"`)
	}
	if info.ContentRange != "" {
		c.Header("Content-Range", info.ContentRange)
		c.Header("Content-Length", strconv.FormatInt(info.Size, 10))
		c.Status(http.StatusPartialContent)
	} else {
		c.Header("Content-Length", strconv.FormatInt(info.Size, 10))
		c.Status(http.StatusOK)
	}
	_, _ = io.Copy(c.Writer, body)
}

func setAssetPublicCORS(c *gin.Context) {
	origin := c.GetHeader("Origin")
	if origin != "" {
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Vary", "Origin")
	} else {
		c.Header("Access-Control-Allow-Origin", "*")
	}
	c.Header("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type, Content-Range, Accept-Ranges")
}
