package handler

import (
	"github.com/anhtuanlc/mediahub/internal/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type RouterDeps struct {
	Logger      *zap.Logger
	Health      *HealthHandler
	System      *SystemHandler
	Setup       *SetupHandler
	Auth     *AuthHandler
	Member   *MemberHandler
	Perm     *PermissionHandler
	Settings *SettingsHandler
	Objects  *ObjectHandler
	Upload   *UploadHandler
	Password *PasswordTransport
	AuthMW   *middleware.AuthMiddleware
	AppURL   string
	AppEnv   string
}

func NewRouter(deps RouterDeps) *gin.Engine {
	if deps.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(deps.Logger))
	r.Use(middleware.CORS(deps.AppURL))

	r.GET("/health", deps.Health.Liveness)

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
					system.GET("/storage", NotImplemented)
					system.GET("/queue", NotImplemented)
					system.GET("/security", NotImplemented)
					system.GET("/stream-analytics", NotImplemented)
					system.POST("/cleanup/temp", deps.System.CleanupTemp)
					system.POST("/cleanup/orphans", deps.System.CleanupOrphans)
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

			protected.GET("/videos", NotImplemented)
			protected.GET("/videos/:public_id", NotImplemented)
			protected.POST("/videos/:public_id/convert", NotImplemented)
			protected.GET("/videos/:public_id/hls", NotImplemented)
			protected.DELETE("/videos/:public_id/hls", NotImplemented)

			protected.GET("/api-keys", NotImplemented)
			protected.POST("/api-keys", NotImplemented)
			protected.PATCH("/api-keys/:public_id", NotImplemented)
			protected.DELETE("/api-keys/:public_id", NotImplemented)
		}
	}

	stream := r.Group("/stream")
	{
		stream.GET("/:video_public_id/*filepath", StreamForbidden)
	}

	return r
}
