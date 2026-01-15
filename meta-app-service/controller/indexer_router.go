package controller

import (
	"meta-app-service/conf"
	"meta-app-service/controller/handler"
	"meta-app-service/controller/respond"
	"meta-app-service/docs"
	"meta-app-service/service/indexer_service"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// SetupIndexerRouter setup indexer service router
func SetupIndexerRouter(indexerService *indexer_service.IndexerService) *gin.Engine {
	// Set Swagger host from config
	if conf.Cfg.Indexer.SwaggerBaseUrl != "" {
		docs.SwaggerInfo.Host = conf.Cfg.Indexer.SwaggerBaseUrl
	}

	// Create Gin engine
	r := gin.Default()

	// Add CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // Allow all origins, can be configured to specific domains
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH", "HEAD"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization", "Accept", "Cache-Control", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           12 * 3600, // 12 hours
	}))

	// Add timing middleware
	r.Use(respond.TimingMiddleware())

	// Create sync status service instance
	syncStatusService := indexer_service.NewSyncStatusService()
	// Set scanner for getting latest block height
	if indexerService != nil {
		syncStatusService.SetBlockScanner(indexerService.GetScanner())
	}

	// Create handlers
	metaAppHandler := handler.NewMetaAppHandler(syncStatusService)
	tempAppHandler := handler.NewTempAppHandler()

	// API v1 route group
	v1 := r.Group("/api/v1")
	{
		// MetaApp routes
		metaapps := v1.Group("/metaapps")
		{
			// Get MetaApp list (cursor pagination)
			metaapps.GET("", metaAppHandler.ListMetaApps)

			// Get MetaApps by creator MetaID (must be before /first/:firstPinId to avoid route conflict)
			metaapps.GET("/creator/:metaId", metaAppHandler.GetMetaAppsByCreatorMetaID)

			// Get MetaApp history by FirstPinID (must be before /first/:firstPinId to avoid route conflict)
			metaapps.GET("/first/:firstPinId/history", metaAppHandler.GetMetaAppHistoryByFirstPinID)

			// Download MetaApp as zip by FirstPinID (must be before /first/:firstPinId to avoid route conflict)
			metaapps.GET("/first/:firstPinId/download", metaAppHandler.DownloadMetaAppAsZip)

			// Get MetaApp by FirstPinID (must be before /:pinId to avoid route conflict)
			metaapps.GET("/first/:firstPinId", metaAppHandler.GetMetaAppByFirstPinID)

			// Redeploy MetaApp (must be before /:pinId to avoid route conflict)
			metaapps.POST("/:pinId/redeploy", metaAppHandler.RedeployMetaApp)

			// Get MetaApp by PinID
			metaapps.GET("/:pinId", metaAppHandler.GetMetaAppByPinID)
		}

		// Sync status route
		v1.GET("/status", metaAppHandler.GetSyncStatus)

		// Statistics route
		v1.GET("/stats", metaAppHandler.GetStats)

		// Config route
		v1.GET("/config", metaAppHandler.GetConfig)

		// Deploy queue route
		v1.GET("/deploy-queue", metaAppHandler.ListDeployQueue)

		// TempApp routes
		tempapps := v1.Group("/temp-apps")
		{
			// Chunk upload routes (must be before /:tokenId to avoid route conflict)
			chunk := tempapps.Group("/chunk")
			{
				// Initialize chunk upload
				chunk.POST("/init", tempAppHandler.InitChunkUpload)

				// Get chunk upload status
				chunk.GET("/:uploadId/status", tempAppHandler.GetChunkUploadStatus)

				// Merge chunks
				chunk.POST("/:uploadId/merge", tempAppHandler.MergeChunks)

				// Upload chunk
				chunk.POST("/:uploadId/:chunkIndex", tempAppHandler.UploadChunk)
			}

			// Upload temp app zip file
			tempapps.POST("/upload", tempAppHandler.UploadTempApp)

			// Get temp app by tokenId (must be last to avoid route conflict)
			tempapps.GET("/:tokenId", tempAppHandler.GetTempAppByTokenID)
		}
	}

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "indexer",
		})
	})

	// Swagger documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler,
		ginSwagger.InstanceName("swagger")))

	// Static files and web pages - 使用明确的 GET 路由确保优先匹配
	// 这些路由必须在参数路由 /:pinId 之前注册
	r.Static("/static", "./web/static")
	r.GET("/", func(c *gin.Context) {
		c.File("./web/indexer.html")
	})
	r.GET("/indexer.html", func(c *gin.Context) {
		c.File("./web/indexer.html")
	})
	r.GET("/indexer.js", func(c *gin.Context) {
		c.Header("Content-Type", "application/javascript; charset=utf-8")
		c.File("./web/indexer.js")
	})

	// TempApp 静态文件服务路由（必须在 MetaApp 路由之前注册，避免路由冲突）
	// 支持访问 /temp/{tokenId}/index.html 以及 /temp/{tokenId}/*filepath 下的所有静态资源
	r.GET("/temp/:tokenId/*filepath", tempAppHandler.ServeTempAppStaticFiles)
	r.GET("/temp/:tokenId", tempAppHandler.ServeTempAppStaticFiles)

	// MetaApp 静态文件服务路由（必须在所有特定路由之后注册，避免路由冲突）
	// 支持访问 /{pinId}/index.html 以及 /{pinId}/*filepath 下的所有静态资源
	// 注意：只使用通配符路由，避免与特定路由冲突
	r.GET("/:pinId/*filepath", metaAppHandler.ServeMetaAppStaticFiles)

	// 处理 /{pinId} 的直接访问（检查文件是否存在，如果存在则重定向到 /{pinId}/index.html）
	// 如果文件不存在，返回 404
	r.GET("/:pinId", metaAppHandler.ServeMetaAppStaticFiles)

	return r
}
