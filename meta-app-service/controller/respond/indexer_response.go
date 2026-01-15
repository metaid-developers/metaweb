package respond

import (
	"time"

	"meta-app-service/conf"
	model "meta-app-service/models"
	"meta-app-service/service/indexer_service"
)

// IndexerFileResponse file information response structure
type IndexerFileResponse struct {
	// ID             int64     `json:"id" example:"1"`
	PinID         string `json:"pin_id" example:"abc123def456i0"`
	TxID          string `json:"tx_id" example:"abc123def456789"`
	Path          string `json:"path" example:"/file/test.jpg"`
	Operation     string `json:"operation" example:"create"`
	Encryption    string `json:"encryption" example:"0"`
	ContentType   string `json:"content_type" example:"image/jpeg"`
	FileType      string `json:"file_type" example:"image"`
	FileExtension string `json:"file_extension" example:".jpg"`
	FileName      string `json:"file_name" example:"test.jpg"`
	FileSize      int64  `json:"file_size" example:"102400"`
	FileMd5       string `json:"file_md5" example:"d41d8cd98f00b204e9800998ecf8427e"`
	FileHash      string `json:"file_hash" example:"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"`
	// StorageType    string    `json:"storage_type" example:"oss"`
	StoragePath    string `json:"storage_path" example:"indexer/mvc/pinid123i0.jpg"`
	ChainName      string `json:"chain_name" example:"mvc"`
	BlockHeight    int64  `json:"block_height" example:"12345"`
	Timestamp      int64  `json:"timestamp" example:"1699999999"`
	CreatorMetaId  string `json:"creator_meta_id" example:"abc123def456..."`
	CreatorAddress string `json:"creator_address" example:"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"`
	OwnerMetaId    string `json:"owner_meta_id" example:"abc123def456..."`
	OwnerAddress   string `json:"owner_address" example:"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"`
	// Status         string    `json:"status" example:"success"`
	// CreatedAt      time.Time `json:"created_at" example:"2024-01-01T00:00:00Z"`
	// UpdatedAt      time.Time `json:"updated_at" example:"2024-01-01T00:00:00Z"`
}

// IndexerAvatarResponse avatar information response structure
type IndexerAvatarResponse struct {
	// ID            int64     `json:"id" example:"1"`
	PinID         string    `json:"pin_id" example:"xyz789i0"`
	TxID          string    `json:"tx_id" example:"xyz789"`
	MetaId        string    `json:"meta_id" example:"abc123def456..."`
	Address       string    `json:"address" example:"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"`
	Avatar        string    `json:"avatar" example:"indexer/avatar/mvc/xyz789/xyz789i0.jpg"`
	ContentType   string    `json:"content_type" example:"image/jpeg"`
	FileSize      int64     `json:"file_size" example:"102400"`
	FileMd5       string    `json:"file_md5" example:"d41d8cd98f00b204e9800998ecf8427e"`
	FileHash      string    `json:"file_hash" example:"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"`
	FileExtension string    `json:"file_extension" example:".jpg"`
	FileType      string    `json:"file_type" example:"image"`
	ChainName     string    `json:"chain_name" example:"mvc"`
	BlockHeight   int64     `json:"block_height" example:"12345"`
	Timestamp     int64     `json:"timestamp" example:"1699999999"`
	CreatedAt     time.Time `json:"created_at" example:"2024-01-01T00:00:00Z"`
	UpdatedAt     time.Time `json:"updated_at" example:"2024-01-01T00:00:00Z"`
}

// IndexerSyncStatusResponse sync status response structure
type IndexerSyncStatusResponse struct {
	// ID                int64     `json:"id" example:"1"`
	ChainName         string    `json:"chain_name" example:"mvc"`
	CurrentSyncHeight int64     `json:"current_sync_height" example:"12345"`
	LatestBlockHeight int64     `json:"latest_block_height" example:"12350"`
	CreatedAt         time.Time `json:"created_at" example:"2024-01-01T00:00:00Z"`
	UpdatedAt         time.Time `json:"updated_at" example:"2024-01-01T00:00:00Z"`
}

// ToIndexerSyncStatusResponse convert sync status to response
func ToIndexerSyncStatusResponse(status *model.IndexerSyncStatus, latestHeight int64) IndexerSyncStatusResponse {
	if status == nil {
		return IndexerSyncStatusResponse{
			LatestBlockHeight: latestHeight,
		}
	}

	return IndexerSyncStatusResponse{
		ChainName:         status.ChainName,
		CurrentSyncHeight: status.CurrentSyncHeight,
		LatestBlockHeight: latestHeight,
		CreatedAt:         status.CreatedAt,
		UpdatedAt:         status.UpdatedAt,
	}
}

// IndexerFileListResponse file list response structure
type IndexerFileListResponse struct {
	Files      []IndexerFileResponse `json:"files"`
	NextCursor int64                 `json:"next_cursor" example:"100"`
	HasMore    bool                  `json:"has_more" example:"true"`
}

// IndexerAvatarListResponse avatar list response structure
type IndexerAvatarListResponse struct {
	Avatars    []IndexerAvatarResponse `json:"avatars"`
	NextCursor int64                   `json:"next_cursor" example:"100"`
	HasMore    bool                    `json:"has_more" example:"true"`
}

// IndexerStatsResponse statistics response structure
type IndexerStatsResponse struct {
	TotalApps int64 `json:"total_apps" example:"12345"`
}

// ToIndexerStatsResponse convert stats to response
func ToIndexerStatsResponse(totalApps int64) IndexerStatsResponse {
	return IndexerStatsResponse{
		TotalApps: totalApps,
	}
}

// MetaAppResponse MetaApp 响应结构
type MetaAppResponse struct {
	*model.MetaApp
	DeployInfo *model.MetaAppDeployFileContent `json:"deploy_info,omitempty"`
}

// ToMetaAppResponse 转换 MetaAppWithDeploy 为响应结构
func ToMetaAppResponse(app *indexer_service.MetaAppWithDeploy) MetaAppResponse {
	return MetaAppResponse{
		MetaApp:    app.MetaApp,
		DeployInfo: app.DeployInfo,
	}
}

// MetaAppListResponse MetaApp 列表响应结构
type MetaAppListResponse struct {
	Apps       []MetaAppResponse `json:"apps"`
	NextCursor int64             `json:"next_cursor" example:"100"`
	HasMore    bool              `json:"has_more" example:"true"`
}

// ToMetaAppListResponse 转换 MetaApp 列表为响应结构
func ToMetaAppListResponse(apps []*indexer_service.MetaAppWithDeploy, nextCursor int64, hasMore bool) MetaAppListResponse {
	result := make([]MetaAppResponse, 0, len(apps))
	for _, app := range apps {
		result = append(result, ToMetaAppResponse(app))
	}
	return MetaAppListResponse{
		Apps:       result,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}
}

// MetaAppHistoryResponse MetaApp 历史版本列表响应结构
type MetaAppHistoryResponse struct {
	History []MetaAppResponse `json:"history"`
}

// DeployQueueResponse 部署队列响应结构
type DeployQueueResponse struct {
	FirstPinId  string    `json:"first_pin_id"`
	PinID       string    `json:"pin_id"`
	Timestamp   int64     `json:"timestamp"`
	Content     string    `json:"content"`
	Code        string    `json:"code"`
	ContentType string    `json:"content_type"`
	Version     string    `json:"version"`
	TryCount    int       `json:"try_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// ToDeployQueueResponse 转换部署队列为响应结构
func ToDeployQueueResponse(queue *model.MetaAppDeployQueue) DeployQueueResponse {
	if queue == nil {
		return DeployQueueResponse{}
	}
	return DeployQueueResponse{
		FirstPinId:  queue.FirstPinId,
		PinID:       queue.PinID,
		Timestamp:   queue.Timestamp,
		Content:     queue.Content,
		Code:        queue.Code,
		ContentType: queue.ContentType,
		Version:     queue.Version,
		TryCount:    queue.TryCount,
		CreatedAt:   queue.CreatedAt,
	}
}

// DeployQueueListResponse 部署队列列表响应结构
type DeployQueueListResponse struct {
	Queues     []DeployQueueResponse `json:"queues"`
	NextCursor int64                 `json:"next_cursor" example:"100"`
	HasMore    bool                  `json:"has_more" example:"true"`
}

// ToDeployQueueListResponse 转换部署队列列表为响应结构
func ToDeployQueueListResponse(queues []*model.MetaAppDeployQueue, nextCursor int64, hasMore bool) DeployQueueListResponse {
	result := make([]DeployQueueResponse, 0, len(queues))
	for _, queue := range queues {
		result = append(result, ToDeployQueueResponse(queue))
	}
	return DeployQueueListResponse{
		Queues:     result,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}
}

// ConfigResponse 配置信息响应结构
type ConfigResponse struct {
	MetafsDomain string `json:"metafs_domain" example:"http://localhost:7281"`
}

// ToConfigResponse 转换配置为响应结构
func ToConfigResponse() ConfigResponse {
	metafsDomain := "http://localhost:7281" // 默认值
	if conf.Cfg != nil && conf.Cfg.Metafs.Domain != "" {
		metafsDomain = conf.Cfg.Metafs.Domain
	}
	return ConfigResponse{
		MetafsDomain: metafsDomain,
	}
}
