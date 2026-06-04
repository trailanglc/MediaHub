package handler

import (
	"fmt"
	"html"
	"net/http"
	"strconv"
	"time"

	"github.com/anhtuanlc/mediahub/internal/integration"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type EmbedHandler struct {
	Videos   *repository.VideoRepository
	Delivery *service.DeliveryService
	StreamTok *service.StreamTokenService
	StreamBaseURL string
}

func NewEmbedHandler(videos *repository.VideoRepository, delivery *service.DeliveryService, streamTok *service.StreamTokenService, streamBaseURL string) *EmbedHandler {
	return &EmbedHandler{
		Videos:        videos,
		Delivery:      delivery,
		StreamTok:     streamTok,
		StreamBaseURL: streamBaseURL,
	}
}

func (h *EmbedHandler) Serve(c *gin.Context) {
	videoPID, err := uuid.Parse(c.Param("video_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid video id"})
		return
	}
	exp, _ := strconv.ParseInt(c.Query("e"), 10, 64)
	sig := c.Query("s")
	if h.Delivery == nil || exp <= 0 || sig == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "missing signature"})
		return
	}
	if err := h.Delivery.VerifyAsset(videoPID.String(), integration.AssetVariantEmbed, exp, sig); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "invalid or expired signature"})
		return
	}

	row, err := h.Videos.GetByObjectPublicID(c.Request.Context(), videoPID)
	if err != nil || row.Asset.HLSStatus != "ready" {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "video not ready"})
		return
	}

	ttl := time.Hour
	if h.Delivery != nil {
		ttl = h.Delivery.AssetWindow()
	}
	masterURL := h.StreamTok.BuildStreamURL(h.StreamBaseURL, videoPID.String(), "master.m3u8", ttl)
	escURL := html.EscapeString(masterURL)

	c.Header("Cache-Control", "private, no-store")
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, embedPlayerHTML(escURL))
}

func embedPlayerHTML(masterURL string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>MediaHub Player</title>
<style>
html,body{margin:0;height:100%%;background:#000}
video{width:100%%;height:100%%;object-fit:contain;background:#000}
</style>
</head>
<body>
<video id="v" controls playsinline crossorigin="use-credentials"></video>
<script src="https://cdn.jsdelivr.net/npm/hls.js@1.5.17/dist/hls.min.js"></script>
<script>
(function(){
  var src=%q;
  var video=document.getElementById('v');
  if(window.Hls&&Hls.isSupported()){
    var hls=new Hls({enableWorker:true,xhrSetup:function(xhr){xhr.withCredentials=true;},startLevel:-1,capLevelToPlayerSize:true});
    hls.loadSource(src);
    hls.attachMedia(video);
  }else if(video.canPlayType('application/vnd.apple.mpegurl')){
    video.src=src;
  }
})();
</script>
</body>
</html>`, masterURL)
}
