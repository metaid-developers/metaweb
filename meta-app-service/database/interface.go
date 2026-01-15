package database

import (
	model "meta-app-service/models"
)

// Database interface for different database implementations
type Database interface {
	// MetaApp operations
	CreateMetaApp(app *model.MetaApp) error
	GetMetaAppByPinID(pinID string) (*model.MetaApp, error)
	UpdateMetaApp(app *model.MetaApp) error
	GetMetaAppsByCreatorMetaIDWithCursor(metaID string, cursor int64, size int) ([]*model.MetaApp, int64, error)
	ListMetaAppsWithCursor(cursor int64, size int) ([]*model.MetaApp, int64, error)
	CountMetaApps() (int64, error)
	GetLatestMetaAppByFirstPinID(firstPinID string) (*model.MetaApp, error)
	GetMetaAppHistoryByFirstPinID(firstPinID string) ([]*model.MetaApp, error)

	// IndexerSyncStatus operations
	CreateOrUpdateIndexerSyncStatus(status *model.IndexerSyncStatus) error
	GetIndexerSyncStatusByChainName(chainName string) (*model.IndexerSyncStatus, error)
	UpdateIndexerSyncStatusHeight(chainName string, height int64) error
	GetAllIndexerSyncStatus() ([]*model.IndexerSyncStatus, error)

	// MetaApp deploy operations
	AddToDeployQueue(queue *model.MetaAppDeployQueue) error
	GetDeployQueueItem(pinID string) (*model.MetaAppDeployQueue, error)
	UpdateDeployQueueItem(queue *model.MetaAppDeployQueue) error
	RemoveFromDeployQueue(pinID string) error
	GetNextDeployQueueItem() (*model.MetaAppDeployQueue, error)
	ListDeployQueueWithCursor(cursor int64, size int) ([]*model.MetaAppDeployQueue, int64, error)
	CreateOrUpdateDeployFileContent(content *model.MetaAppDeployFileContent) error
	GetDeployFileContent(pinID string) (*model.MetaAppDeployFileContent, error)

	// TempApp deploy operations
	CreateTempAppDeploy(deploy *model.TempAppDeploy) error
	GetTempAppDeployByTokenID(tokenID string) (*model.TempAppDeploy, error)
	DeleteTempAppDeploy(tokenID string) error
	ListExpiredTempAppDeploys() ([]*model.TempAppDeploy, error)

	// TempApp chunk upload operations
	CreateTempAppChunkUpload(upload *model.TempAppChunkUpload) error
	GetTempAppChunkUploadByUploadID(uploadID string) (*model.TempAppChunkUpload, error)
	UpdateTempAppChunkUpload(upload *model.TempAppChunkUpload) error
	DeleteTempAppChunkUpload(uploadID string) error

	// General operations
	Close() error
}

// DBType database type
type DBType string

const (
	DBTypeMySQL  DBType = "mysql"
	DBTypePebble DBType = "pebble"
)

// Global database instance
var DB Database

// currentDBType stores the current database type
var currentDBType DBType

// InitDatabase initialize database with specified type
func InitDatabase(dbType DBType, config interface{}) error {
	var err error

	switch dbType {
	case DBTypePebble:
		DB, err = NewPebbleDatabase(config)
		currentDBType = DBTypePebble
	default:
		return ErrUnsupportedDBType
	}

	return err
}

// GetGormDB get GORM database instance (only for MySQL)
func GetGormDB() interface{} {
	return nil
}
