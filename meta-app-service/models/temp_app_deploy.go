package models

import "time"

// TempAppDeploy 临时应用部署模型
type TempAppDeploy struct {
	TokenID        string    `json:"token_id"`         // 唯一临时 token
	DeployFilePath string    `json:"deploy_file_path"` // 部署文件路径
	ExpiresAt      time.Time `json:"expires_at"`       // 过期时间
	Status         string    `json:"status"`           // 状态: pending/processing/completed/failed
	Message        string    `json:"message"`          // 错误信息等
	CreatedAt      time.Time `json:"created_at"`       // 创建时间
	UpdatedAt      time.Time `json:"updated_at"`       // 更新时间
}

// TempAppChunkUpload 临时应用分片上传模型
type TempAppChunkUpload struct {
	UploadID       string       `json:"upload_id"`       // 上传 ID（UUID）
	TokenID        string       `json:"token_id"`        // 临时应用 TokenID（合并后生成）
	TotalSize      int64        `json:"total_size"`      // 总文件大小
	TotalChunks    int          `json:"total_chunks"`    // 总分片数
	ChunkSize      int64        `json:"chunk_size"`      // 分片大小
	UploadedChunks map[int]bool `json:"uploaded_chunks"` // 已上传的分片索引（key: chunkIndex, value: true）
	Status         string       `json:"status"`          // 状态: uploading/merging/completed/failed
	Message        string       `json:"message"`         // 错误信息等
	CreatedAt      time.Time    `json:"created_at"`      // 创建时间
	UpdatedAt      time.Time    `json:"updated_at"`      // 更新时间
}
