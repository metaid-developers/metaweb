package indexer_service

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"meta-app-service/conf"
	"meta-app-service/database"
	model "meta-app-service/models"
	"meta-app-service/models/dao"
)

// IndexerAppService MetaApp 查询服务
type IndexerAppService struct {
	metaAppDAO *dao.MetaAppDAO
}

// NewIndexerAppService 创建 MetaApp 查询服务实例
func NewIndexerAppService() *IndexerAppService {
	return &IndexerAppService{
		metaAppDAO: dao.NewMetaAppDAO(),
	}
}

// MetaAppWithDeploy MetaApp 带部署信息
type MetaAppWithDeploy struct {
	*model.MetaApp
	DeployInfo *model.MetaAppDeployFileContent `json:"deploy_info,omitempty"`
}

// ListMetaApps 获取 MetaApp 列表（时间倒序，可分页）
// cursor: 游标（从 0 开始）
// size: 每页大小
func (s *IndexerAppService) ListMetaApps(cursor, size int64) ([]*MetaAppWithDeploy, int64, error) {
	if s.metaAppDAO == nil {
		return nil, 0, database.ErrDatabaseNotInitialized
	}

	// 获取 MetaApp 列表（从 collectionMetaAppTimestamp，返回每个 first_pin_id 的最新版本）
	apps, nextCursor, err := s.metaAppDAO.ListWithCursor(cursor, int(size))
	if err != nil {
		return nil, 0, err
	}

	// 获取每个 MetaApp 的部署信息（使用 first_pin_id）
	result := make([]*MetaAppWithDeploy, 0, len(apps))
	for _, app := range apps {
		appWithDeploy := &MetaAppWithDeploy{
			MetaApp: app,
		}

		// 获取部署信息
		deployPinID := app.PinID
		// if deployPinID == "" {
		// 	deployPinID = app.PinID
		// }
		deployInfo, err := database.DB.GetDeployFileContent(deployPinID)
		if err == nil && deployInfo != nil {
			appWithDeploy.DeployInfo = deployInfo
		}

		result = append(result, appWithDeploy)
	}

	return result, nextCursor, nil
}

// GetMetaAppsByCreatorMetaID 根据 MetaID 获取 MetaApp 列表（包括部署情况，时间倒序，可分页）
// metaID: 创建者 MetaID
// cursor: 游标（从 0 开始）
// size: 每页大小
func (s *IndexerAppService) GetMetaAppsByCreatorMetaID(metaID string, cursor, size int64) ([]*MetaAppWithDeploy, int64, error) {
	if s.metaAppDAO == nil {
		return nil, 0, database.ErrDatabaseNotInitialized
	}

	// 获取 MetaApp 列表（从 collectionMetaAppMetaIDTimestamp，返回每个 first_pin_id 的最新版本）
	apps, nextCursor, err := s.metaAppDAO.GetByCreatorMetaIDWithCursor(metaID, cursor, int(size))
	if err != nil {
		return nil, 0, err
	}

	// 获取每个 MetaApp 的部署信息（使用 first_pin_id）
	result := make([]*MetaAppWithDeploy, 0, len(apps))
	for _, app := range apps {
		appWithDeploy := &MetaAppWithDeploy{
			MetaApp: app,
		}

		// 获取部署信息
		deployPinID := app.PinID
		// if deployPinID == "" {
		// 	deployPinID = app.PinID
		// }
		deployInfo, err := database.DB.GetDeployFileContent(deployPinID)
		if err == nil && deployInfo != nil {
			appWithDeploy.DeployInfo = deployInfo
		}

		result = append(result, appWithDeploy)
	}

	return result, nextCursor, nil
}

// GetMetaAppByPinID 根据 PinID 获取 MetaApp 详情（包括部署情况）
// pinID: MetaApp PinID
func (s *IndexerAppService) GetMetaAppByPinID(pinID string) (*MetaAppWithDeploy, error) {
	if s.metaAppDAO == nil {
		return nil, database.ErrDatabaseNotInitialized
	}

	// 获取 MetaApp
	app, err := s.metaAppDAO.GetByPinID(pinID)
	if err != nil {
		return nil, err
	}

	appWithDeploy := &MetaAppWithDeploy{
		MetaApp: app,
	}

	// 获取部署信息（使用 first_pin_id 对应的部署信息，如果有的话）
	deployPinID := pinID
	// if app.FirstPinId != "" {
	// 	// 尝试使用 first_pin_id 获取部署信息（因为部署是基于 first_pin_id 的）
	// 	deployInfo, err := database.DB.GetDeployFileContent(app.FirstPinId)
	// 	if err == nil && deployInfo != nil {
	// 		appWithDeploy.DeployInfo = deployInfo
	// 		return appWithDeploy, nil
	// 	}
	// }

	// 如果 first_pin_id 没有部署信息，尝试使用当前 pinID
	deployInfo, err := database.DB.GetDeployFileContent(deployPinID)
	if err == nil && deployInfo != nil {
		appWithDeploy.DeployInfo = deployInfo
	}

	return appWithDeploy, nil
}

// GetMetaAppByFirstPinID 根据 FirstPinID 获取最新的 MetaApp 详情（包括部署情况）
// firstPinID: MetaApp FirstPinID
func (s *IndexerAppService) GetMetaAppByFirstPinID(firstPinID string) (*MetaAppWithDeploy, error) {
	if s.metaAppDAO == nil {
		return nil, database.ErrDatabaseNotInitialized
	}

	// 获取最新的 MetaApp
	app, err := database.DB.GetLatestMetaAppByFirstPinID(firstPinID)
	if err != nil {
		return nil, err
	}

	appWithDeploy := &MetaAppWithDeploy{
		MetaApp: app,
	}

	// 获取部署信息
	deployInfo, err := database.DB.GetDeployFileContent(app.PinID)
	if err == nil && deployInfo != nil {
		appWithDeploy.DeployInfo = deployInfo
	}

	return appWithDeploy, nil
}

// GetMetaAppHistoryByFirstPinID 根据 FirstPinID 获取 MetaApp 历史版本列表
// firstPinID: MetaApp FirstPinID
func (s *IndexerAppService) GetMetaAppHistoryByFirstPinID(firstPinID string) ([]*MetaAppWithDeploy, error) {
	if s.metaAppDAO == nil {
		return nil, database.ErrDatabaseNotInitialized
	}

	// 获取历史记录
	history, err := database.DB.GetMetaAppHistoryByFirstPinID(firstPinID)
	if err != nil {
		return nil, err
	}

	// 转换为带部署信息的列表
	result := make([]*MetaAppWithDeploy, 0, len(history))
	for _, app := range history {
		appWithDeploy := &MetaAppWithDeploy{
			MetaApp: app,
		}

		// 获取部署信息（使用 first_pin_id）
		deployInfo, err := database.DB.GetDeployFileContent(app.PinID)
		if err == nil && deployInfo != nil {
			appWithDeploy.DeployInfo = deployInfo
		}

		result = append(result, appWithDeploy)
	}

	return result, nil
}

// GetStats 获取统计信息（当前已同步的 MetaApp 总数）
func (s *IndexerAppService) GetStats() (int64, error) {
	if s.metaAppDAO == nil {
		return 0, database.ErrDatabaseNotInitialized
	}

	// 获取 MetaApp 总数
	count, err := database.DB.CountMetaApps()
	if err != nil {
		return 0, err
	}

	return count, nil
}

// RedeployMetaApp 根据 PinID 重新将 MetaApp 加入部署队列
// pinID: MetaApp PinID
func (s *IndexerAppService) RedeployMetaApp(pinID string) error {
	if s.metaAppDAO == nil {
		return database.ErrDatabaseNotInitialized
	}

	// 1. 获取 MetaApp 信息
	metaApp, err := s.metaAppDAO.GetByPinID(pinID)
	if err != nil {
		return fmt.Errorf("failed to get MetaApp: %w", err)
	}

	fristMetaApp, err := database.DB.GetLatestMetaAppByFirstPinID(metaApp.FirstPinId)
	if err != nil {
		return err
	}

	deployPinId := fristMetaApp.PinID

	// 2. 检查是否已经在队列中，如果在则返回错误
	existingQueueItem, err := database.DB.GetDeployQueueItem(deployPinId)
	if err == nil && existingQueueItem != nil {
		// 已经在队列中，返回错误
		return fmt.Errorf("MetaApp %s is already in deploy queue", deployPinId)
	}

	// 3. 准备部署队列项
	codePinID := fristMetaApp.Code
	if codePinID == "" {
		// 如果没有 Code，尝试使用 Content
		if fristMetaApp.Content != "" {
			if !strings.HasPrefix(fristMetaApp.Content, "metafile://") {
				codePinID = "metafile://" + fristMetaApp.Content
			} else {
				codePinID = fristMetaApp.Content
			}
		}
	}

	if codePinID == "" {
		return fmt.Errorf("no code or content pinId found for MetaApp %s", deployPinId)
	}

	// 4. 创建新的部署队列项（重置 TryCount 为 0）
	queue := &model.MetaAppDeployQueue{
		PinID:       fristMetaApp.PinID,
		Timestamp:   fristMetaApp.Timestamp,
		Content:     fristMetaApp.Content,
		Code:        codePinID,
		ContentType: fristMetaApp.ContentType,
		Version:     fristMetaApp.Version,
		TryCount:    0, // 重置重试次数
		CreatedAt:   time.Now(),
	}

	// 5. 添加到部署队列
	if err := database.DB.AddToDeployQueue(queue); err != nil {
		return fmt.Errorf("failed to add to deploy queue: %w", err)
	}

	return nil
}

// DownloadMetaAppAsZip 根据 FirstPinID 压缩对应的部署文件夹为 zip 文件
// firstPinID: MetaApp FirstPinID
// 返回 zip 文件路径和错误
func (s *IndexerAppService) DownloadMetaAppAsZip(firstPinID string) (string, error) {
	if firstPinID == "" {
		return "", fmt.Errorf("firstPinID is required")
	}

	// 获取部署基础目录
	deployBaseDir := conf.Cfg.MetaApp.DeployFilePath
	if deployBaseDir == "" {
		deployBaseDir = "./meta_app_deploy_data"
	}

	// 构建应用部署目录路径
	appDeployDir := filepath.Join(deployBaseDir, firstPinID)

	// 检查目录是否存在
	info, err := os.Stat(appDeployDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("deploy directory not found for firstPinID: %s", firstPinID)
		}
		return "", fmt.Errorf("failed to access deploy directory: %w", err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", appDeployDir)
	}

	// 创建临时 zip 文件
	tmpDir := os.TempDir()
	zipFileName := fmt.Sprintf("%s.zip", firstPinID)
	zipFilePath := filepath.Join(tmpDir, zipFileName)

	// 创建 zip 文件
	zipFile, err := os.Create(zipFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	// 创建 zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// 遍历目录并添加到 zip
	err = filepath.Walk(appDeployDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 获取相对路径（相对于 appDeployDir）
		relPath, err := filepath.Rel(appDeployDir, filePath)
		if err != nil {
			return err
		}

		// 跳过根目录本身
		if relPath == "." {
			return nil
		}

		// 创建 zip 文件头
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// 设置文件名（使用相对路径，保持目录结构）
		header.Name = relPath

		// 如果是目录，设置目录标志
		if info.IsDir() {
			header.Name += "/"
		} else {
			// 设置压缩方法
			header.Method = zip.Deflate
		}

		// 写入文件头
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		// 如果是文件，复制文件内容
		if !info.IsDir() {
			file, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(writer, file)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		// 清理临时文件
		os.Remove(zipFilePath)
		return "", fmt.Errorf("failed to create zip: %w", err)
	}

	return zipFilePath, nil
}
