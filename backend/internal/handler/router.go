package handler

import (
	"github.com/anhtuanlc/mediahub/internal/middleware"
	"github.com/anhtuanlc/mediahub/internal/observability"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type RouterDeps struct {
	Logger      *zap.Logger
	Health      *HealthHandler
	SystemInfo  *SystemInfoHandler
	System      *SystemHandler
	Queue       *QueueHandler
	Setup       *SetupHandler
	Auth        *AuthHandler
	Member      *MemberHandler
	Perm        *PermissionHandler
	Settings    *SettingsHandler
	Objects     *ObjectHandler
	Upload      *UploadHandler
	Videos      *VideoHandler
	APIKeys     *APIKeyHandler
	Stream      *StreamHandler
	Password    *PasswordTransport
	AuthMW      *middleware.AuthMiddleware
	AppURL          string
	AppEnv          string
	TrustedProxies  []string
}

func NewRouter(deps RouterDeps) *gin.Engine {
	if deps.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	if err := r.SetTrustedProxies(deps.TrustedProxies); err != nil {
		panic("trusted proxies: " + err.Error())
	}
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(deps.Logger))
	r.Use(middleware.CORS(deps.AppURL))
	r.Use(observability.HTTPMetricsMiddleware())

	r.GET("/health", deps.Health.Liveness)
	r.GET("/metrics", observability.PrometheusHandler())

	api := r.Group("/api")
	{
		setup := api.Group("/setup")
		{
			setup.GET("/status", deps.Setup.Status)
			setup.POST("/owner", deps.Setup.CreateOwner)
		}

		api.GET("/auth/crypto/public-key", deps.Password.PublicKey)

		authPublic := api.Group("/auth")
		{
			authPublic.POST("/login", deps.Auth.Login)
			authPublic.POST("/refresh", deps.Auth.Refresh)
		}

		protected := api.Group("")
		protected.Use(deps.AuthMW.RequireAuth())
		{
			protected.GET("/auth/me", deps.Auth.Me)
			protected.POST("/auth/logout", deps.Auth.Logout)
			protected.GET("/permissions/mine", deps.Perm.ListMine)

			owner := protected.Group("")
			owner.Use(middleware.RequireOwner())
			{
				owner.GET("/members", deps.Member.List)
				owner.POST("/members", deps.Member.Create)
				owner.GET("/members/:public_id", deps.Member.Get)
				owner.PATCH("/members/:public_id", deps.Member.Update)
				owner.DELETE("/members/:public_id", deps.Member.Delete)
				owner.POST("/members/:public_id/restore", deps.Member.Restore)
				owner.DELETE("/members/:public_id/purge", deps.Member.Purge)

				owner.GET("/permissions", deps.Perm.List)
				owner.POST("/permissions", deps.Perm.Grant)
				owner.DELETE("/permissions/:id", deps.Perm.Revoke)

				owner.GET("/settings", deps.Settings.Get)
				owner.PATCH("/settings", deps.Settings.Update)

				system := owner.Group("/system")
				{
					system.GET("/health", deps.Health.SystemHealth)
					system.GET("/health/:component", deps.Health.ComponentHealth)
					if deps.SystemInfo != nil {
						system.GET("/storage", deps.SystemInfo.Storage)
						system.GET("/security", deps.SystemInfo.Security)
					} else {
						system.GET("/storage", NotImplemented)
						system.GET("/security", NotImplemented)
					}
					if deps.Queue != nil {
						system.GET("/queue", deps.Queue.QueueStatus)
						system.GET("/stream-analytics", deps.Queue.StreamAnalytics)
					} else {
						system.GET("/queue", NotImplemented)
						system.GET("/stream-analytics", NotImplemented)
					}
					system.POST("/cleanup/temp", deps.System.CleanupTemp)
					system.POST("/cleanup/orphans", deps.System.CleanupOrphans)
				}

				if deps.APIKeys != nil {
					owner.GET("/api-keys", deps.APIKeys.List)
					owner.POST("/api-keys", deps.APIKeys.Create)
					owner.PATCH("/api-keys/:public_id", deps.APIKeys.Patch)
					owner.DELETE("/api-keys/:public_id", deps.APIKeys.Delete)
				}
			}

			protected.GET("/objects", deps.Objects.List)
			protected.GET("/objects/trash", deps.Objects.ListTrash)
			protected.POST("/objects/trash/empty", deps.Objects.EmptyTrash)
			protected.GET("/objects/search", deps.Objects.Search)
			protected.POST("/objects/preview-urls", deps.Objects.BatchPreviewURLs)
			protected.POST("/objects/bulk-rename", deps.Objects.BulkRename)
			protected.POST("/folders", deps.Objects.CreateFolder)
			protected.POST("/objects/:public_id/restore", deps.Objects.Restore)
			protected.DELETE("/objects/:public_id/purge", deps.Objects.Purge)
			protected.GET("/objects/:public_id", deps.Objects.Get)
			protected.PATCH("/objects/:public_id", deps.Objects.Patch)
			protected.DELETE("/objects/:public_id", deps.Objects.Delete)

			protected.GET("/upload/limits", deps.Upload.Limits)
			protected.POST("/upload/init", deps.Upload.Init)
			protected.PUT("/upload/:session_id/chunks/:index", deps.Upload.PutChunk)
			protected.POST("/upload/:session_id/complete", deps.Upload.Complete)
			protected.DELETE("/upload/:session_id", deps.Upload.Abort)

			if deps.Videos != nil {
				protected.GET("/videos", deps.Videos.List)
				protected.GET("/videos/:public_id", deps.Videos.Get)
				protected.POST("/videos/:public_id/convert", deps.Videos.Convert)
				protected.GET("/videos/:public_id/hls", deps.Videos.GetHLS)
				protected.DELETE("/videos/:public_id/hls", deps.Videos.DeleteHLS)
				protected.GET("/videos/:public_id/stream-policy", deps.Videos.GetStreamPolicy)
				protected.PATCH("/videos/:public_id/stream-policy", deps.Videos.PatchStreamPolicy)
			}
		}
	}

	if deps.Stream != nil {
		stream := r.Group("/stream")
		{
			stream.GET("/:video_public_id/*filepath", deps.Stream.Serve)
		}
	}

	return r
}
