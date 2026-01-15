package respond

import (
	"time"

	"meta-app-service/conf"
	model "meta-app-service/models"
)

// TempAppDeploymentDetails 临时应用部署详情
type TempAppDeploymentDetails struct {
	DeployFilePath string `json:"deploy_file_path"` // 部署文件路径
	Status         string `json:"status"`           // 状态
	Message        string `json:"message"`          // 消息
}

// TempAppDeployResponse 临时应用部署响应结构
type TempAppDeployResponse struct {
	ID                string                    `json:"id"`                 // TokenID
	URL               string                    `json:"url"`                // 相对路径 URL
	PreviewURL        string                    `json:"preview_url"`        // 预览 URL（完整 URL）
	ExpiresAt         time.Time                 `json:"expires_at"`         // 过期时间
	DeploymentDetails *TempAppDeploymentDetails `json:"deployment_details"` // 部署详情
}

// ToTempAppDeployResponse 转换 TempAppDeploy 为响应结构
func ToTempAppDeployResponse(deploy *model.TempAppDeploy) TempAppDeployResponse {
	// 构建 URL
	url := "/temp/" + deploy.TokenID

	// 构建预览 URL
	previewURL := url
	if conf.Cfg != nil && conf.Cfg.Indexer.SwaggerBaseUrl != "" {
		// 如果配置了基础 URL，使用 https 协议
		previewURL = "https://" + conf.Cfg.Indexer.SwaggerBaseUrl + url
	}

	return TempAppDeployResponse{
		ID:         deploy.TokenID,
		URL:        url,
		PreviewURL: previewURL,
		ExpiresAt:  deploy.ExpiresAt,
		DeploymentDetails: &TempAppDeploymentDetails{
			DeployFilePath: deploy.DeployFilePath,
			Status:         deploy.Status,
			Message:        deploy.Message,
		},
	}
}

// TempAppChunkInitResponse 分片上传初始化响应结构
type TempAppChunkInitResponse struct {
	UploadID    string `json:"upload_id"`    // 上传 ID
	ChunkSize   int64  `json:"chunk_size"`   // 分片大小
	TotalChunks int    `json:"total_chunks"` // 总分片数
}

// ToTempAppChunkInitResponse 转换 TempAppChunkUpload 为初始化响应结构
func ToTempAppChunkInitResponse(upload *model.TempAppChunkUpload) TempAppChunkInitResponse {
	return TempAppChunkInitResponse{
		UploadID:    upload.UploadID,
		ChunkSize:   upload.ChunkSize,
		TotalChunks: upload.TotalChunks,
	}
}

// TempAppChunkUploadResponse 分片上传状态响应结构
type TempAppChunkUploadResponse struct {
	UploadID       string  `json:"upload_id"`       // 上传 ID
	TokenID        string  `json:"token_id"`        // 临时应用 TokenID（合并后生成）
	TotalSize      int64   `json:"total_size"`      // 总文件大小
	TotalChunks    int     `json:"total_chunks"`    // 总分片数
	ChunkSize      int64   `json:"chunk_size"`      // 分片大小
	UploadedChunks []int   `json:"uploaded_chunks"` // 已上传的分片索引列表
	Status         string  `json:"status"`          // 状态: uploading/merging/completed/failed
	Message        string  `json:"message"`         // 错误信息等
	Progress       float64 `json:"progress"`        // 上传进度（0-100）
}

// ToTempAppChunkUploadResponse 转换 TempAppChunkUpload 为响应结构
func ToTempAppChunkUploadResponse(upload *model.TempAppChunkUpload) TempAppChunkUploadResponse {
	// 计算已上传的分片索引列表
	uploadedChunks := make([]int, 0, len(upload.UploadedChunks))
	for chunkIndex := range upload.UploadedChunks {
		uploadedChunks = append(uploadedChunks, chunkIndex)
	}

	// 计算上传进度
	var progress float64
	if upload.TotalChunks > 0 {
		progress = float64(len(upload.UploadedChunks)) / float64(upload.TotalChunks) * 100
	}

	return TempAppChunkUploadResponse{
		UploadID:       upload.UploadID,
		TokenID:        upload.TokenID,
		TotalSize:      upload.TotalSize,
		TotalChunks:    upload.TotalChunks,
		ChunkSize:      upload.ChunkSize,
		UploadedChunks: uploadedChunks,
		Status:         upload.Status,
		Message:        upload.Message,
		Progress:       progress,
	}
}
