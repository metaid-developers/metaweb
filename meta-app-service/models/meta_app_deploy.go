package models

import "time"

// MetaAppDeployQueue MetaApp 部署队列模型
type MetaAppDeployQueue struct {
	FirstPinId  string    `json:"first_pin_id"` // 第一个 PIN ID
	PinID       string    `json:"pin_id"`       // MetaApp PinID
	Timestamp   int64     `json:"timestamp"`    // 时间戳（用于排序）
	Content     string    `json:"content"`      // Content pinId
	Code        string    `json:"code"`         // Code pinId (metafile://pinid)
	ContentType string    `json:"content_type"` // 内容类型
	Version     string    `json:"version"`      // 版本号
	TryCount    int       `json:"try_count"`    // 重试次数
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
}

// MetaAppDeployFileContent MetaApp 部署文件内容模型
type MetaAppDeployFileContent struct {
	FirstPinId     string    `json:"first_pin_id"`     // 第一个 PIN ID
	PinID          string    `json:"pin_id"`           // MetaApp PinID
	Content        string    `json:"content"`          // Content pinId
	Code           string    `json:"code"`             // Code pinId
	ContentType    string    `json:"content_type"`     // 内容类型
	Version        string    `json:"version"`          // 版本号
	DeployStatus   string    `json:"deploy_status"`    // 部署状态: pending/processing/completed/failed
	DeployFilePath string    `json:"deploy_file_path"` // 部署文件路径
	DeployMessage  string    `json:"deploy_message"`   // 部署消息（错误信息等）
	CreatedAt      time.Time `json:"created_at"`       // 创建时间
	UpdatedAt      time.Time `json:"updated_at"`       // 更新时间
}
