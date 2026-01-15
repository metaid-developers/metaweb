package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"meta-app-service/conf"
	"meta-app-service/controller/respond"
	"meta-app-service/database"
	"meta-app-service/service/indexer_service"

	"github.com/gin-gonic/gin"
)

// MetaAppHandler MetaApp 查询处理器
type MetaAppHandler struct {
	appService        *indexer_service.IndexerAppService
	syncStatusService *indexer_service.SyncStatusService
}

// NewMetaAppHandler 创建 MetaApp 查询处理器实例
func NewMetaAppHandler(syncStatusService *indexer_service.SyncStatusService) *MetaAppHandler {
	return &MetaAppHandler{
		appService:        indexer_service.NewIndexerAppService(),
		syncStatusService: syncStatusService,
	}
}

// ListMetaApps 获取 MetaApp 列表（时间倒序，可分页）
// @Summary 获取 MetaApp 列表
// @Description 获取所有 MetaApp 列表，按时间倒序排列，支持分页
// @Tags MetaApp
// @Accept json
// @Produce json
// @Param cursor query int false "游标（从 0 开始）" default(0)
// @Param size query int false "每页大小" default(20)
// @Success 200 {object} respond.Response{data=respond.MetaAppListResponse}
// @Router /api/v1/metaapps [get]
func (h *MetaAppHandler) ListMetaApps(c *gin.Context) {
	// 解析查询参数
	cursor, _ := strconv.ParseInt(c.DefaultQuery("cursor", "0"), 10, 64)
	size, _ := strconv.ParseInt(c.DefaultQuery("size", "20"), 10, 64)

	// 限制每页大小
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	// 调用服务
	apps, nextCursor, err := h.appService.ListMetaApps(cursor, size)
	if err != nil {
		if err == database.ErrNotFound {
			respond.NotFound(c, "no metaapps found")
			return
		}
		respond.ServerError(c, err.Error())
		return
	}

	// 构建响应
	hasMore := nextCursor > cursor+int64(len(apps))
	response := respond.ToMetaAppListResponse(apps, nextCursor, hasMore)

	respond.Success(c, response)
}

// GetMetaAppsByCreatorMetaID 根据 MetaID 获取 MetaApp 列表（包括部署情况，时间倒序，可分页）
// @Summary 根据 MetaID 获取 MetaApp 列表
// @Description 根据创建者 MetaID 获取 MetaApp 列表，包括部署情况，按时间倒序排列，支持分页
// @Tags MetaApp
// @Accept json
// @Produce json
// @Param metaId path string true "创建者 MetaID"
// @Param cursor query int false "游标（从 0 开始）" default(0)
// @Param size query int false "每页大小" default(20)
// @Success 200 {object} respond.Response{data=respond.MetaAppListResponse}
// @Router /api/v1/metaapps/creator/{metaId} [get]
func (h *MetaAppHandler) GetMetaAppsByCreatorMetaID(c *gin.Context) {
	metaID := c.Param("metaId")
	if metaID == "" {
		respond.InvalidParam(c, "metaId is required")
		return
	}

	// 解析查询参数
	cursor, _ := strconv.ParseInt(c.DefaultQuery("cursor", "0"), 10, 64)
	size, _ := strconv.ParseInt(c.DefaultQuery("size", "20"), 10, 64)

	// 限制每页大小
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	// 调用服务
	apps, nextCursor, err := h.appService.GetMetaAppsByCreatorMetaID(metaID, cursor, size)
	if err != nil {
		if err == database.ErrNotFound {
			respond.NotFound(c, "no metaapps found for this metaId")
			return
		}
		respond.ServerError(c, err.Error())
		return
	}

	// 构建响应
	hasMore := nextCursor > cursor+int64(len(apps))
	response := respond.ToMetaAppListResponse(apps, nextCursor, hasMore)

	respond.Success(c, response)
}

// GetMetaAppByPinID 根据 PinID 获取 MetaApp 详情（包括部署情况）
// @Summary 根据 PinID 获取 MetaApp 详情
// @Description 根据 PinID 获取 MetaApp 详细信息，包括部署情况
// @Tags MetaApp
// @Accept json
// @Produce json
// @Param pinId path string true "MetaApp PinID"
// @Success 200 {object} respond.Response{data=indexer_service.MetaAppWithDeploy}
// @Router /api/v1/metaapps/{pinId} [get]
func (h *MetaAppHandler) GetMetaAppByPinID(c *gin.Context) {
	pinID := c.Param("pinId")
	if pinID == "" {
		respond.InvalidParam(c, "pinId is required")
		return
	}

	// 调用服务
	app, err := h.appService.GetMetaAppByPinID(pinID)
	if err != nil {
		if err == database.ErrNotFound {
			respond.NotFound(c, "metaapp not found")
			return
		}
		respond.ServerError(c, err.Error())
		return
	}

	respond.Success(c, respond.ToMetaAppResponse(app))
}

// GetSyncStatus 获取同步状态
// @Summary 获取同步状态
// @Description 获取索引器同步状态（包括从节点获取的最新区块高度）
// @Tags Indexer Status
// @Accept json
// @Produce json
// @Success 200 {object} respond.Response{data=respond.IndexerSyncStatusResponse}
// @Failure 500 {object} respond.Response
// @Router /api/v1/status [get]
func (h *MetaAppHandler) GetSyncStatus(c *gin.Context) {
	if h.syncStatusService == nil {
		respond.ServerError(c, "sync status service not available")
		return
	}

	status, err := h.syncStatusService.GetSyncStatus()
	if err != nil {
		respond.ServerError(c, err.Error())
		return
	}

	// Get latest block height from node
	latestHeight, err := h.syncStatusService.GetLatestBlockHeight()
	if err != nil {
		// If failed to get from node, use 0 as fallback
		latestHeight = 0
	}

	respond.Success(c, respond.ToIndexerSyncStatusResponse(status, latestHeight))
}

// GetStats 获取统计信息
// @Summary 获取统计信息
// @Description 获取索引器统计信息（当前已同步的 MetaApp 总数）
// @Tags Indexer Status
// @Accept json
// @Produce json
// @Success 200 {object} respond.Response{data=respond.IndexerStatsResponse}
// @Failure 500 {object} respond.Response
// @Router /api/v1/stats [get]
func (h *MetaAppHandler) GetStats(c *gin.Context) {
	// Get total MetaApp count
	totalApps, err := h.appService.GetStats()
	if err != nil {
		respond.ServerError(c, err.Error())
		return
	}

	respond.Success(c, respond.ToIndexerStatsResponse(totalApps))
}

// GetConfig 获取配置信息（包括 Metafs Domain 等前端需要的配置）
// @Summary 获取配置信息
// @Description 获取前端需要的配置信息，如 Metafs Domain
// @Tags Indexer Status
// @Accept json
// @Produce json
// @Success 200 {object} respond.Response{data=respond.ConfigResponse}
// @Failure 500 {object} respond.Response
// @Router /api/v1/config [get]
func (h *MetaAppHandler) GetConfig(c *gin.Context) {
	respond.Success(c, respond.ToConfigResponse())
}

// RedeployMetaApp 重新部署 MetaApp
// @Summary 重新部署 MetaApp
// @Description 根据 PinID 重新将 MetaApp 加入部署队列
// @Tags MetaApp
// @Accept json
// @Produce json
// @Param pinId path string true "MetaApp PinID"
// @Success 200 {object} respond.Response
// @Failure 400 {object} respond.Response
// @Failure 404 {object} respond.Response
// @Failure 500 {object} respond.Response
// @Router /api/v1/metaapps/{pinId}/redeploy [post]
func (h *MetaAppHandler) RedeployMetaApp(c *gin.Context) {
	pinID := c.Param("pinId")
	if pinID == "" {
		respond.InvalidParam(c, "pinId is required")
		return
	}

	// 调用服务重新部署
	err := h.appService.RedeployMetaApp(pinID)
	if err != nil {
		// 检查是否是已在队列中的错误
		if strings.Contains(err.Error(), "already in deploy queue") {
			respond.Error(c, respond.CodeInvalidParam, err.Error())
			return
		}
		// 检查是否是未找到的错误
		if err == database.ErrNotFound || strings.Contains(err.Error(), "not found") {
			respond.NotFound(c, "metaapp not found")
			return
		}
		respond.ServerError(c, err.Error())
		return
	}

	respond.SuccessWithMsg(c, "MetaApp added to deploy queue successfully", nil)
}

// GetMetaAppByFirstPinID 根据 FirstPinID 获取最新的 MetaApp 详情（包括部署情况）
// @Summary 根据 FirstPinID 获取最新的 MetaApp 详情
// @Description 根据 FirstPinID 获取最新的 MetaApp 详细信息，包括部署情况
// @Tags MetaApp
// @Accept json
// @Produce json
// @Param firstPinId path string true "MetaApp FirstPinID"
// @Success 200 {object} respond.Response{data=respond.MetaAppResponse}
// @Router /api/v1/metaapps/first/{firstPinId} [get]
func (h *MetaAppHandler) GetMetaAppByFirstPinID(c *gin.Context) {
	firstPinID := c.Param("firstPinId")
	if firstPinID == "" {
		respond.InvalidParam(c, "firstPinId is required")
		return
	}

	// 调用服务
	app, err := h.appService.GetMetaAppByFirstPinID(firstPinID)
	if err != nil {
		if err == database.ErrNotFound {
			respond.NotFound(c, "metaapp not found")
			return
		}
		respond.ServerError(c, err.Error())
		return
	}

	respond.Success(c, respond.ToMetaAppResponse(app))
}

// DownloadMetaAppAsZip 根据 FirstPinID 下载 MetaApp 部署文件为 zip
// @Summary 下载 MetaApp 部署文件为 zip
// @Description 根据 FirstPinID 压缩对应的部署文件夹并下载为 zip 文件
// @Tags MetaApp
// @Accept json
// @Produce application/zip
// @Param firstPinId path string true "MetaApp FirstPinID"
// @Success 200 {file} file "zip file"
// @Failure 400 {object} respond.Response
// @Failure 404 {object} respond.Response
// @Failure 500 {object} respond.Response
// @Router /api/v1/metaapps/first/{firstPinId}/download [get]
func (h *MetaAppHandler) DownloadMetaAppAsZip(c *gin.Context) {
	firstPinID := c.Param("firstPinId")
	if firstPinID == "" {
		respond.InvalidParam(c, "firstPinId is required")
		return
	}

	// 调用服务生成 zip 文件
	zipFilePath, err := h.appService.DownloadMetaAppAsZip(firstPinID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respond.NotFound(c, err.Error())
			return
		}
		respond.ServerError(c, err.Error())
		return
	}

	// 确保文件存在
	if _, err := os.Stat(zipFilePath); os.IsNotExist(err) {
		respond.NotFound(c, "zip file not found")
		return
	}

	// 设置响应头
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.zip\"", firstPinID))

	// 发送文件
	c.File(zipFilePath)

	// 延迟删除临时文件
	go func() {
		// 等待一小段时间确保文件已发送
		time.Sleep(5 * time.Second)
		os.Remove(zipFilePath)
	}()
}

// GetMetaAppHistoryByFirstPinID 根据 FirstPinID 获取 MetaApp 历史版本列表
// @Summary 根据 FirstPinID 获取 MetaApp 历史版本列表
// @Description 根据 FirstPinID 获取 MetaApp 的所有历史版本列表
// @Tags MetaApp
// @Accept json
// @Produce json
// @Param firstPinId path string true "MetaApp FirstPinID"
// @Success 200 {object} respond.Response{data=respond.MetaAppHistoryResponse}
// @Router /api/v1/metaapps/first/{firstPinId}/history [get]
func (h *MetaAppHandler) GetMetaAppHistoryByFirstPinID(c *gin.Context) {
	firstPinID := c.Param("firstPinId")
	if firstPinID == "" {
		respond.InvalidParam(c, "firstPinId is required")
		return
	}

	// 调用服务
	history, err := h.appService.GetMetaAppHistoryByFirstPinID(firstPinID)
	if err != nil {
		if err == database.ErrNotFound {
			respond.NotFound(c, "metaapp history not found")
			return
		}
		respond.ServerError(c, err.Error())
		return
	}

	// 转换为响应结构
	result := make([]respond.MetaAppResponse, 0, len(history))
	for _, app := range history {
		result = append(result, respond.ToMetaAppResponse(app))
	}

	respond.Success(c, respond.MetaAppHistoryResponse{
		History: result,
	})
}

// ListDeployQueue 获取部署队列列表（支持游标分页）
// @Summary 获取部署队列列表
// @Description 获取部署队列列表，按时间戳倒序排列，支持分页
// @Tags Deploy Queue
// @Accept json
// @Produce json
// @Param cursor query int false "游标（从 0 开始）" default(0)
// @Param size query int false "每页大小" default(20)
// @Success 200 {object} respond.Response{data=respond.DeployQueueListResponse}
// @Failure 500 {object} respond.Response
// @Router /api/v1/deploy-queue [get]
func (h *MetaAppHandler) ListDeployQueue(c *gin.Context) {
	// 解析查询参数
	cursor, _ := strconv.ParseInt(c.DefaultQuery("cursor", "0"), 10, 64)
	size, _ := strconv.ParseInt(c.DefaultQuery("size", "20"), 10, 64)

	// 限制每页大小
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	// 调用数据库接口
	if database.DB == nil {
		respond.ServerError(c, "database not initialized")
		return
	}

	queues, nextCursor, err := database.DB.ListDeployQueueWithCursor(cursor, int(size))
	if err != nil {
		if err == database.ErrNotFound {
			respond.NotFound(c, "no deploy queue items found")
			return
		}
		respond.ServerError(c, err.Error())
		return
	}

	// 构建响应
	hasMore := nextCursor > cursor+int64(len(queues))
	response := respond.ToDeployQueueListResponse(queues, nextCursor, hasMore)

	respond.Success(c, response)
}

// ServeMetaAppStaticFiles 提供 MetaApp 部署的静态文件服务
// 支持访问 /{pinId}/index.html 以及 /{pinId}/*filepath 下的所有静态资源
func (h *MetaAppHandler) ServeMetaAppStaticFiles(c *gin.Context) {
	pinID := c.Param("pinId")
	if pinID == "" {
		respond.NotFound(c, "pinId is required")
		return
	}

	// 排除已知的静态文件路径，这些应该由其他路由处理
	// 例如: indexer.js, indexer.html, static, swagger, api, health 等
	excludedPaths := []string{
		"indexer.js", "indexer.html", "static", "swagger", "api", "health",
		"favicon.ico", "robots.txt",
	}
	for _, excluded := range excludedPaths {
		if pinID == excluded {
			respond.NotFound(c, "not found")
			return
		}
	}

	// 验证 pinID 格式：64 个十六进制字符 + 'i' + 数字
	// 例如: 5ea55a16ce4ecc795101f564b8c4f2e77aacddd2b256f031498d855432893530i0
	matched, err := regexp.MatchString(`^[0-9a-f]{64}i\d+$`, pinID)
	if err != nil || !matched {
		respond.NotFound(c, "invalid pinId format")
		return
	}

	// 获取文件路径（如果请求的是 /{pinId}/index.html，filepath 会是 "/index.html"）
	// 如果请求的是 /{pinId}，filepath 会是空字符串
	requestedFilePath := c.Param("filepath")

	// 移除前导斜杠（如果存在）
	requestedFilePath = strings.TrimPrefix(requestedFilePath, "/")

	// 获取部署基础目录
	deployBaseDir := conf.Cfg.MetaApp.DeployFilePath
	if deployBaseDir == "" {
		deployBaseDir = "./meta_app_deploy_data"
	}

	// 构建应用部署目录
	appDeployDir := filepath.Join(deployBaseDir, pinID)

	// 检查应用部署目录是否存在
	if _, err := os.Stat(appDeployDir); os.IsNotExist(err) {
		fmt.Printf("[ServeMetaAppStaticFiles] App directory not found: %s\n", appDeployDir)
		respond.NotFound(c, "metaapp not deployed")
		return
	}

	// 如果没有指定文件路径（即访问 /{pinId} 而不是 /{pinId}/），
	// 则重定向到带斜杠的版本，这样可以确保浏览器的基础路径正确
	// 避免前端资源路径解析错误
	if requestedFilePath == "" {
		// 获取完整的请求路径
		fullPath := c.Request.URL.Path
		// 如果路径不以斜杠结尾，重定向到带斜杠的版本
		if !strings.HasSuffix(fullPath, "/") {
			// 301 永久重定向到带斜杠的版本
			pathPrefix := getPathPrefix(c)
			c.Redirect(301, pathPrefix+fullPath+"/")
			fmt.Printf("[ServeMetaAppStaticFiles] Redirecting to: %s\n", fullPath+"/")
			return
		}
		// 如果已经有斜杠（即访问 /{pinId}/），则使用 index.html
		fmt.Printf("[ServeMetaAppStaticFiles] Serving index.html for pinID: %s\n", pinID)
	}

	// 确定要服务的文件路径
	filePath := requestedFilePath
	if filePath == "" {
		filePath = "index.html"
	} else {
		fmt.Printf("[ServeMetaAppStaticFiles] Requested filepath: %s for pinID: %s\n", filePath, pinID)
	}

	// 构建完整的文件路径
	fullFilePath := filepath.Join(appDeployDir, filePath)

	// 安全检查：防止路径遍历攻击
	// 确保请求的文件路径在部署目录内
	cleanDeployDir := filepath.Clean(appDeployDir)
	cleanFilePath := filepath.Clean(fullFilePath)
	if !strings.HasPrefix(cleanFilePath, cleanDeployDir+string(os.PathSeparator)) && cleanFilePath != cleanDeployDir {
		respond.NotFound(c, "invalid file path")
		return
	}

	// 检查文件是否存在
	fileInfo, err := os.Stat(cleanFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			respond.NotFound(c, "file not found")
			return
		}
		respond.ServerError(c, "failed to access file")
		return
	}

	// 如果是目录，返回 404
	if fileInfo.IsDir() {
		respond.NotFound(c, "file not found")
		return
	}

	// 设置正确的 Content-Type（根据文件扩展名）
	// 这样可以避免浏览器自动重定向
	contentType := getContentType(cleanFilePath)
	if contentType != "" {
		c.Header("Content-Type", contentType)
	}

	// 直接返回文件内容，不重定向
	// 使用 c.File() 但确保不会重定向
	c.File(cleanFilePath)
}

// getPathPrefix 获取路径前缀，优先级：配置 > X-Forwarded-Prefix 请求头 > 空字符串
func getPathPrefix(c *gin.Context) string {
	// 1. 优先使用配置
	if conf.Cfg != nil && conf.Cfg.Indexer.PathPrefix != "" {
		return conf.Cfg.Indexer.PathPrefix
	}

	// 2. 其次使用请求头（反向代理常用）
	if prefix := c.GetHeader("X-Forwarded-Prefix"); prefix != "" {
		return prefix
	}

	// 3. 默认返回空字符串
	return ""
}

// getContentType 根据文件扩展名返回 Content-Type
func getContentType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".woff", ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".eot":
		return "application/vnd.ms-fontobject"
	default:
		return ""
	}
}
