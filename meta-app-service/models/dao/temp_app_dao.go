package dao

import (
	"fmt"

	"meta-app-service/database"
	model "meta-app-service/models"
)

// TempAppDAO 临时应用 DAO
type TempAppDAO struct {
	db database.Database
}

// NewTempAppDAO 创建临时应用 DAO 实例
func NewTempAppDAO() *TempAppDAO {
	return &TempAppDAO{
		db: database.DB,
	}
}

// Create 创建临时应用部署记录
func (d *TempAppDAO) Create(deploy *model.TempAppDeploy) error {
	if d.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return d.db.CreateTempAppDeploy(deploy)
}

// GetByTokenID 根据 TokenID 获取临时应用部署记录
func (d *TempAppDAO) GetByTokenID(tokenID string) (*model.TempAppDeploy, error) {
	if d.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return d.db.GetTempAppDeployByTokenID(tokenID)
}

// Delete 删除临时应用部署记录
func (d *TempAppDAO) Delete(tokenID string) error {
	if d.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return d.db.DeleteTempAppDeploy(tokenID)
}

// ListExpired 获取所有过期的临时应用部署记录
func (d *TempAppDAO) ListExpired() ([]*model.TempAppDeploy, error) {
	if d.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return d.db.ListExpiredTempAppDeploys()
}

// CreateChunkUpload 创建临时应用分片上传记录
func (d *TempAppDAO) CreateChunkUpload(upload *model.TempAppChunkUpload) error {
	if d.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return d.db.CreateTempAppChunkUpload(upload)
}

// GetChunkUploadByUploadID 根据 UploadID 获取临时应用分片上传记录
func (d *TempAppDAO) GetChunkUploadByUploadID(uploadID string) (*model.TempAppChunkUpload, error) {
	if d.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return d.db.GetTempAppChunkUploadByUploadID(uploadID)
}

// UpdateChunkUpload 更新临时应用分片上传记录
func (d *TempAppDAO) UpdateChunkUpload(upload *model.TempAppChunkUpload) error {
	if d.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return d.db.UpdateTempAppChunkUpload(upload)
}

// DeleteChunkUpload 删除临时应用分片上传记录
func (d *TempAppDAO) DeleteChunkUpload(uploadID string) error {
	if d.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return d.db.DeleteTempAppChunkUpload(uploadID)
}
