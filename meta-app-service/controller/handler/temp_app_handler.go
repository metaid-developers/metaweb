package handler

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"meta-app-service/conf"
	"meta-app-service/controller/respond"
	"meta-app-service/database"
	"meta-app-service/service/temp_deploy_service"

	"github.com/gin-gonic/gin"
)

// TempAppHandler 临时应用处理器
type TempAppHandler struct {
	tempDeployService *temp_deploy_service.TempDeployService
}

// NewTempAppHandler 创建临时应用处理器实例
func NewTempAppHandler() *TempAppHandler {
	return &TempAppHandler{
		tempDeployService: temp_deploy_service.NewTempDeployService(),
	}
}

// checkTempAppEnabled 检查临时应用功能是否启用
func (h *TempAppHandler) checkTempAppEnabled(c *gin.Context) bool {
	if conf.Cfg == nil || !conf.Cfg.TempApp.Enable {
		respond.Error(c, respond.CodeInvalidParam, "temp app feature is disabled")
		return false
	}
	return true
}

// UploadTempApp 上传临时应用 zip 包
// @Summary 上传临时应用 zip 包
// @Description 上传 zip 包，生成唯一 tokenId，解压并保存，返回部署信息
// @Tags TempApp
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "zip 文件"
// @Success 200 {object} respond.Response{data=respond.TempAppDeployResponse}
// @Failure 400 {object} respond.Response
// @Failure 500 {object} respond.Response
// @Router /api/v1/temp-apps/upload [post]
func (h *TempAppHandler) UploadTempApp(c *gin.Context) {
	// 检查功能是否启用
	if !h.checkTempAppEnabled(c) {
		return
	}

	// 获取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		respond.InvalidParam(c, "file is required")
		return
	}

	// 验证文件扩展名
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".zip") {
		respond.InvalidParam(c, "file must be a zip file")
		return
	}

	// 打开文件
	src, err := file.Open()
	if err != nil {
		respond.ServerError(c, fmt.Sprintf("failed to open file: %v", err))
		return
	}
	defer src.Close()

	// 调用服务上传并解压
	deploy, err := h.tempDeployService.UploadTempApp(src, file.Filename)
	if err != nil {
		respond.ServerError(c, err.Error())
		return
	}

	// 转换为响应结构
	response := respond.ToTempAppDeployResponse(deploy)
	respond.Success(c, response)
}

// GetTempAppByTokenID 根据 TokenID 获取临时应用部署信息
// @Summary 根据 TokenID 获取临时应用部署信息
// @Description 根据 TokenID 获取临时应用的部署信息
// @Tags TempApp
// @Accept json
// @Produce json
// @Param tokenId path string true "临时应用 TokenID"
// @Success 200 {object} respond.Response{data=respond.TempAppDeployResponse}
// @Failure 404 {object} respond.Response
// @Failure 500 {object} respond.Response
// @Router /api/v1/temp-apps/{tokenId} [get]
func (h *TempAppHandler) GetTempAppByTokenID(c *gin.Context) {
	// 检查功能是否启用
	if !h.checkTempAppEnabled(c) {
		return
	}

	tokenID := c.Param("tokenId")
	if tokenID == "" {
		respond.InvalidParam(c, "tokenId is required")
		return
	}

	// 调用服务查询
	deploy, err := h.tempDeployService.GetTempAppByTokenID(tokenID)
	if err != nil {
		if err == database.ErrNotFound {
			respond.NotFound(c, "temp app not found")
			return
		}
		respond.ServerError(c, err.Error())
		return
	}

	// 转换为响应结构
	response := respond.ToTempAppDeployResponse(deploy)
	respond.Success(c, response)
}

// ServeTempAppStaticFiles 提供临时应用部署的静态文件服务
// 支持访问 /temp/{tokenId}/index.html 以及 /temp/{tokenId}/*filepath 下的所有静态资源
func (h *TempAppHandler) ServeTempAppStaticFiles(c *gin.Context) {
	// 检查功能是否启用
	if !h.checkTempAppEnabled(c) {
		return
	}

	tokenID := c.Param("tokenId")
	if tokenID == "" {
		respond.NotFound(c, "tokenId is required")
		return
	}

	log.Println("tokenID", tokenID)

	// 获取文件路径（如果请求的是 /temp/{tokenId}/index.html，filepath 会是 "/index.html"）
	// 如果请求的是 /temp/{tokenId}，filepath 会是空字符串
	requestedFilePath := c.Param("filepath")

	log.Println("requestedFilePath", requestedFilePath)

	// 移除前导斜杠（如果存在）
	requestedFilePath = strings.TrimPrefix(requestedFilePath, "/")

	log.Println("requestedFilePath after trim", requestedFilePath)

	// 获取部署基础目录
	deployBaseDir := conf.Cfg.TempApp.DeployFilePath
	if deployBaseDir == "" {
		deployBaseDir = "./temp_app_deploy_data"
	}

	// 构建应用部署目录
	appDeployDir := filepath.Join(deployBaseDir, tokenID)

	log.Println("appDeployDir", appDeployDir)

	// 检查应用部署目录是否存在
	if _, err := os.Stat(appDeployDir); os.IsNotExist(err) {
		respond.NotFound(c, "temp app not found")
		return
	}

	// 如果没有指定文件路径（即访问 /temp/{tokenId} 而不是 /temp/{tokenId}/），
	// 则重定向到带斜杠的版本
	if requestedFilePath == "" {
		// 获取完整的请求路径
		fullPath := c.Request.URL.Path
		// 如果路径不以斜杠结尾，重定向到带斜杠的版本
		if !strings.HasSuffix(fullPath, "/") {
			// 301 永久重定向到带斜杠的版本
			pathPrefix := getPathPrefix(c)
			c.Redirect(301, pathPrefix+fullPath+"/")
			return
		}
		// 如果已经有斜杠（即访问 /temp/{tokenId}/），则使用 index.html
	}

	log.Println("requestedFilePath", requestedFilePath)

	// 确定要服务的文件路径
	filePath := requestedFilePath
	if filePath == "" {
		filePath = "index.html"
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
	contentType := getContentType(cleanFilePath)
	if contentType != "" {
		c.Header("Content-Type", contentType)
	}

	// 直接返回文件内容
	c.File(cleanFilePath)
}

// InitChunkUpload 初始化分片上传
// @Summary 初始化分片上传
// @Description 初始化分片上传，返回 uploadId 和分片信息
// @Tags TempApp
// @Accept json
// @Produce json
// @Param total_size formData int true "文件总大小（字节）"
// @Param filename formData string false "文件名"
// @Success 200 {object} respond.Response{data=respond.TempAppChunkInitResponse}
// @Failure 400 {object} respond.Response
// @Failure 500 {object} respond.Response
// @Router /api/v1/temp-apps/chunk/init [post]
func (h *TempAppHandler) InitChunkUpload(c *gin.Context) {
	// 检查功能是否启用
	if !h.checkTempAppEnabled(c) {
		return
	}

	// 获取参数
	totalSizeStr := c.PostForm("total_size")
	if totalSizeStr == "" {
		respond.InvalidParam(c, "total_size is required")
		return
	}

	totalSize, err := strconv.ParseInt(totalSizeStr, 10, 64)
	if err != nil || totalSize <= 0 {
		respond.InvalidParam(c, "invalid total_size")
		return
	}

	filename := c.PostForm("filename")

	// 调用服务初始化分片上传
	upload, err := h.tempDeployService.InitChunkUpload(totalSize, filename)
	if err != nil {
		respond.ServerError(c, err.Error())
		return
	}

	// 转换为响应结构
	response := respond.ToTempAppChunkInitResponse(upload)
	respond.Success(c, response)
}

// UploadChunk 上传分片
// @Summary 上传分片
// @Description 上传单个分片数据
// @Tags TempApp
// @Accept multipart/form-data
// @Produce json
// @Param uploadId path string true "上传 ID"
// @Param chunkIndex path int true "分片索引（从 0 开始）"
// @Param chunk formData file true "分片数据"
// @Success 200 {object} respond.Response
// @Failure 400 {object} respond.Response
// @Failure 500 {object} respond.Response
// @Router /api/v1/temp-apps/chunk/{uploadId}/{chunkIndex} [post]
func (h *TempAppHandler) UploadChunk(c *gin.Context) {
	// 检查功能是否启用
	if !h.checkTempAppEnabled(c) {
		return
	}

	uploadID := c.Param("uploadId")
	if uploadID == "" {
		respond.InvalidParam(c, "uploadId is required")
		return
	}

	chunkIndexStr := c.Param("chunkIndex")
	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil || chunkIndex < 0 {
		respond.InvalidParam(c, "invalid chunkIndex")
		return
	}

	// 获取分片数据
	file, err := c.FormFile("chunk")
	if err != nil {
		respond.InvalidParam(c, "chunk is required")
		return
	}

	// 打开文件
	src, err := file.Open()
	if err != nil {
		respond.ServerError(c, fmt.Sprintf("failed to open chunk file: %v", err))
		return
	}
	defer src.Close()

	// 调用服务上传分片
	if err := h.tempDeployService.UploadChunk(uploadID, chunkIndex, src); err != nil {
		respond.ServerError(c, err.Error())
		return
	}

	respond.Success(c, gin.H{"message": "chunk uploaded successfully"})
}

// MergeChunks 合并分片
// @Summary 合并分片
// @Description 合并所有分片并解压，创建临时应用部署
// @Tags TempApp
// @Accept json
// @Produce json
// @Param uploadId path string true "上传 ID"
// @Success 200 {object} respond.Response{data=respond.TempAppDeployResponse}
// @Failure 400 {object} respond.Response
// @Failure 500 {object} respond.Response
// @Router /api/v1/temp-apps/chunk/{uploadId}/merge [post]
func (h *TempAppHandler) MergeChunks(c *gin.Context) {
	// 检查功能是否启用
	if !h.checkTempAppEnabled(c) {
		return
	}

	uploadID := c.Param("uploadId")
	if uploadID == "" {
		respond.InvalidParam(c, "uploadId is required")
		return
	}

	// 调用服务合并分片
	deploy, err := h.tempDeployService.MergeChunks(uploadID)
	if err != nil {
		respond.ServerError(c, err.Error())
		return
	}

	// 转换为响应结构
	response := respond.ToTempAppDeployResponse(deploy)
	respond.Success(c, response)
}

// GetChunkUploadStatus 获取分片上传状态
// @Summary 获取分片上传状态
// @Description 查询分片上传的状态和进度
// @Tags TempApp
// @Accept json
// @Produce json
// @Param uploadId path string true "上传 ID"
// @Success 200 {object} respond.Response{data=respond.TempAppChunkUploadResponse}
// @Failure 404 {object} respond.Response
// @Failure 500 {object} respond.Response
// @Router /api/v1/temp-apps/chunk/{uploadId}/status [get]
func (h *TempAppHandler) GetChunkUploadStatus(c *gin.Context) {
	// 检查功能是否启用
	if !h.checkTempAppEnabled(c) {
		return
	}

	uploadID := c.Param("uploadId")
	if uploadID == "" {
		respond.InvalidParam(c, "uploadId is required")
		return
	}

	// 调用服务查询状态
	upload, err := h.tempDeployService.GetChunkUploadStatus(uploadID)
	if err != nil {
		if err == database.ErrNotFound {
			respond.NotFound(c, "chunk upload not found")
			return
		}
		respond.ServerError(c, err.Error())
		return
	}

	// 转换为响应结构
	response := respond.ToTempAppChunkUploadResponse(upload)
	respond.Success(c, response)
}
