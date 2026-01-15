package dao

import (
	"fmt"

	"meta-app-service/database"
	model "meta-app-service/models"
)

// MetaAppDAO MetaApp DAO
type MetaAppDAO struct {
	db database.Database
}

// NewMetaAppDAO 创建 MetaApp DAO 实例
func NewMetaAppDAO() *MetaAppDAO {
	return &MetaAppDAO{
		db: database.DB,
	}
}

// Create 创建 MetaApp 记录
func (d *MetaAppDAO) Create(app *model.MetaApp) error {
	if d.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return d.db.CreateMetaApp(app)
}

// GetByPinID 根据 PinID 获取 MetaApp
func (d *MetaAppDAO) GetByPinID(pinID string) (*model.MetaApp, error) {
	if d.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return d.db.GetMetaAppByPinID(pinID)
}

// Update 更新 MetaApp 记录
func (d *MetaAppDAO) Update(app *model.MetaApp) error {
	if d.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return d.db.UpdateMetaApp(app)
}

// GetByCreatorMetaIDWithCursor 根据创建者 MetaID 获取 MetaApp 列表（按时间倒序，支持分页）
func (d *MetaAppDAO) GetByCreatorMetaIDWithCursor(metaID string, cursor int64, size int) ([]*model.MetaApp, int64, error) {
	if d.db == nil {
		return nil, 0, fmt.Errorf("database not initialized")
	}
	return d.db.GetMetaAppsByCreatorMetaIDWithCursor(metaID, cursor, size)
}

// ListWithCursor 获取所有 MetaApp 列表（按时间倒序，支持分页）
func (d *MetaAppDAO) ListWithCursor(cursor int64, size int) ([]*model.MetaApp, int64, error) {
	if d.db == nil {
		return nil, 0, fmt.Errorf("database not initialized")
	}
	return d.db.ListMetaAppsWithCursor(cursor, size)
}
