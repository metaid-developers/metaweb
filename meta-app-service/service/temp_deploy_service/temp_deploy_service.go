package temp_deploy_service

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"meta-app-service/conf"
	model "meta-app-service/models"
	"meta-app-service/models/dao"
	"meta-app-service/tool"
)

// TempDeployService 临时应用部署服务
type TempDeployService struct {
	tempAppDAO *dao.TempAppDAO
}

// NewTempDeployService 创建临时应用部署服务实例
func NewTempDeployService() *TempDeployService {
	return &TempDeployService{
		tempAppDAO: dao.NewTempAppDAO(),
	}
}

// UploadTempApp 上传并解压临时应用 zip 包
// file: 上传的 zip 文件
// 返回 TempAppDeploy 和错误
func (s *TempDeployService) UploadTempApp(file io.Reader, filename string) (*model.TempAppDeploy, error) {
	// 1. 生成唯一 tokenID
	tokenID, err := tool.GetUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokenID: %w", err)
	}
	// 将 UUID 中的连字符替换为下划线，使其更适合作为文件夹名
	tokenID = strings.ReplaceAll(tokenID, "-", "_")

	// 2. 获取部署基础目录
	deployBaseDir := conf.Cfg.TempApp.DeployFilePath
	if deployBaseDir == "" {
		deployBaseDir = "./temp_app_deploy_data"
	}

	// 3. 创建应用部署目录
	appDeployDir := filepath.Join(deployBaseDir, tokenID)
	if err := os.MkdirAll(appDeployDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create deploy directory: %w", err)
	}

	// 4. 保存 zip 文件
	zipFilePath := filepath.Join(appDeployDir, "upload.zip")
	zipFile, err := os.Create(zipFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	// 复制文件内容
	if _, err := io.Copy(zipFile, file); err != nil {
		os.RemoveAll(appDeployDir) // 清理目录
		return nil, fmt.Errorf("failed to save zip file: %w", err)
	}
	zipFile.Close()

	// 5. 解压 zip 文件
	if err := s.extractZip(zipFilePath, appDeployDir); err != nil {
		os.RemoveAll(appDeployDir) // 清理目录
		return nil, fmt.Errorf("failed to extract zip file: %w", err)
	}

	// 6. 删除 zip 文件（解压后不再需要）
	os.Remove(zipFilePath)

	// 7. 计算过期时间
	expireHours := conf.Cfg.TempApp.ExpireHours
	if expireHours == 0 {
		expireHours = 24 // 默认 24 小时
	}
	expiresAt := time.Now().Add(time.Duration(expireHours) * time.Hour)

	// 8. 创建数据库记录
	deploy := &model.TempAppDeploy{
		TokenID:        tokenID,
		DeployFilePath: appDeployDir,
		ExpiresAt:      expiresAt,
		Status:         "completed",
		Message:        "",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.tempAppDAO.Create(deploy); err != nil {
		os.RemoveAll(appDeployDir) // 清理目录
		return nil, fmt.Errorf("failed to save deploy record: %w", err)
	}

	return deploy, nil
}

// extractZip 解压 zip 文件到目标目录
func (s *TempDeployService) extractZip(zipPath, destDir string) error {
	// 打开 zip 文件
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	// 遍历 zip 文件中的所有文件
	for _, f := range r.File {
		// 构建目标文件路径
		fpath := filepath.Join(destDir, f.Name)

		// 安全检查：防止路径遍历攻击
		if !strings.HasPrefix(fpath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", f.Name)
		}

		// 如果是目录，创建目录
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, 0755); err != nil {
				return err
			}
			continue
		}

		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return err
		}

		// 打开 zip 中的文件
		rc, err := f.Open()
		if err != nil {
			return err
		}

		// 创建目标文件
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		// 复制文件内容
		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

// GetTempAppByTokenID 根据 TokenID 获取临时应用部署记录
func (s *TempDeployService) GetTempAppByTokenID(tokenID string) (*model.TempAppDeploy, error) {
	return s.tempAppDAO.GetByTokenID(tokenID)
}

// CleanupExpiredTempApps 清理过期的临时应用
// 删除数据库记录和对应的文件夹
func (s *TempDeployService) CleanupExpiredTempApps() error {
	// 获取所有过期的记录
	expired, err := s.tempAppDAO.ListExpired()
	if err != nil {
		return fmt.Errorf("failed to list expired temp apps: %w", err)
	}

	// 删除每个过期的记录和文件夹
	for _, deploy := range expired {
		// 删除文件夹
		if deploy.DeployFilePath != "" {
			if err := os.RemoveAll(deploy.DeployFilePath); err != nil {
				// 记录错误但继续处理其他记录
				fmt.Printf("Failed to remove directory %s: %v\n", deploy.DeployFilePath, err)
			}
		}

		// 删除数据库记录
		if err := s.tempAppDAO.Delete(deploy.TokenID); err != nil {
			// 记录错误但继续处理其他记录
			fmt.Printf("Failed to delete record %s: %v\n", deploy.TokenID, err)
		}
	}

	return nil
}

// InitChunkUpload 初始化分片上传
// totalSize: 文件总大小（字节）
// filename: 文件名
// 返回 TempAppChunkUpload 和错误
func (s *TempDeployService) InitChunkUpload(totalSize int64, filename string) (*model.TempAppChunkUpload, error) {
	// 1. 生成唯一 uploadID
	uploadID, err := tool.GetUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate uploadID: %w", err)
	}
	// 将 UUID 中的连字符替换为下划线
	uploadID = strings.ReplaceAll(uploadID, "-", "_")

	// 2. 获取分片大小
	chunkSize := conf.Cfg.TempApp.ChunkSize
	if chunkSize == 0 {
		chunkSize = 5 * 1024 * 1024 // 默认 5MB
	}

	// 3. 计算总分片数
	totalChunks := int((totalSize + chunkSize - 1) / chunkSize) // 向上取整

	// 4. 获取部署基础目录
	deployBaseDir := conf.Cfg.TempApp.DeployFilePath
	if deployBaseDir == "" {
		deployBaseDir = "./temp_app_deploy_data"
	}

	// 5. 创建分片临时目录
	chunksDir := filepath.Join(deployBaseDir, "chunks", uploadID)
	if err := os.MkdirAll(chunksDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create chunks directory: %w", err)
	}

	// 6. 创建分片上传记录
	upload := &model.TempAppChunkUpload{
		UploadID:       uploadID,
		TokenID:        "", // 合并后生成
		TotalSize:      totalSize,
		TotalChunks:    totalChunks,
		ChunkSize:      chunkSize,
		UploadedChunks: make(map[int]bool),
		Status:         "uploading",
		Message:        "",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.tempAppDAO.CreateChunkUpload(upload); err != nil {
		os.RemoveAll(chunksDir) // 清理目录
		return nil, fmt.Errorf("failed to create chunk upload record: %w", err)
	}

	return upload, nil
}

// UploadChunk 上传分片
// uploadID: 上传 ID
// chunkIndex: 分片索引（从 0 开始）
// chunkData: 分片数据
// 返回错误
func (s *TempDeployService) UploadChunk(uploadID string, chunkIndex int, chunkData io.Reader) error {
	// 1. 获取分片上传记录
	upload, err := s.tempAppDAO.GetChunkUploadByUploadID(uploadID)
	if err != nil {
		return fmt.Errorf("failed to get chunk upload record: %w", err)
	}

	// 2. 验证分片索引
	if chunkIndex < 0 || chunkIndex >= upload.TotalChunks {
		return fmt.Errorf("invalid chunk index: %d, total chunks: %d", chunkIndex, upload.TotalChunks)
	}

	// 3. 获取部署基础目录
	deployBaseDir := conf.Cfg.TempApp.DeployFilePath
	if deployBaseDir == "" {
		deployBaseDir = "./temp_app_deploy_data"
	}

	// 4. 构建分片文件路径
	chunksDir := filepath.Join(deployBaseDir, "chunks", uploadID)
	chunkFilePath := filepath.Join(chunksDir, fmt.Sprintf("chunk_%d", chunkIndex))

	// 5. 保存分片文件（支持覆盖，实现断点续传）
	chunkFile, err := os.Create(chunkFilePath)
	if err != nil {
		return fmt.Errorf("failed to create chunk file: %w", err)
	}
	defer chunkFile.Close()

	// 6. 复制分片数据
	if _, err := io.Copy(chunkFile, chunkData); err != nil {
		os.Remove(chunkFilePath) // 清理失败的分片
		return fmt.Errorf("failed to save chunk data: %w", err)
	}
	chunkFile.Close()

	// 7. 更新已上传分片记录
	upload.UploadedChunks[chunkIndex] = true
	upload.UpdatedAt = time.Now()

	if err := s.tempAppDAO.UpdateChunkUpload(upload); err != nil {
		return fmt.Errorf("failed to update chunk upload record: %w", err)
	}

	return nil
}

// MergeChunks 合并分片并解压
// uploadID: 上传 ID
// 返回 TempAppDeploy 和错误
func (s *TempDeployService) MergeChunks(uploadID string) (*model.TempAppDeploy, error) {
	// 1. 获取分片上传记录
	upload, err := s.tempAppDAO.GetChunkUploadByUploadID(uploadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chunk upload record: %w", err)
	}

	// 2. 检查所有分片是否已上传
	if len(upload.UploadedChunks) != upload.TotalChunks {
		return nil, fmt.Errorf("not all chunks uploaded: %d/%d", len(upload.UploadedChunks), upload.TotalChunks)
	}

	// 3. 验证所有分片索引
	for i := 0; i < upload.TotalChunks; i++ {
		if !upload.UploadedChunks[i] {
			return nil, fmt.Errorf("chunk %d is missing", i)
		}
	}

	// 4. 更新状态为 merging
	upload.Status = "merging"
	upload.UpdatedAt = time.Now()
	if err := s.tempAppDAO.UpdateChunkUpload(upload); err != nil {
		return nil, fmt.Errorf("failed to update chunk upload status: %w", err)
	}

	// 5. 获取部署基础目录
	deployBaseDir := conf.Cfg.TempApp.DeployFilePath
	if deployBaseDir == "" {
		deployBaseDir = "./temp_app_deploy_data"
	}

	// 6. 构建路径
	chunksDir := filepath.Join(deployBaseDir, "chunks", uploadID)
	zipFilePath := filepath.Join(chunksDir, "merged.zip")

	// 7. 合并分片为完整 zip 文件
	zipFile, err := os.Create(zipFilePath)
	if err != nil {
		upload.Status = "failed"
		upload.Message = fmt.Sprintf("failed to create zip file: %v", err)
		s.tempAppDAO.UpdateChunkUpload(upload)
		return nil, fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	// 按顺序合并所有分片
	for i := 0; i < upload.TotalChunks; i++ {
		chunkFilePath := filepath.Join(chunksDir, fmt.Sprintf("chunk_%d", i))
		chunkFile, err := os.Open(chunkFilePath)
		if err != nil {
			upload.Status = "failed"
			upload.Message = fmt.Sprintf("failed to open chunk %d: %v", i, err)
			s.tempAppDAO.UpdateChunkUpload(upload)
			return nil, fmt.Errorf("failed to open chunk %d: %w", i, err)
		}

		if _, err := io.Copy(zipFile, chunkFile); err != nil {
			chunkFile.Close()
			upload.Status = "failed"
			upload.Message = fmt.Sprintf("failed to merge chunk %d: %v", i, err)
			s.tempAppDAO.UpdateChunkUpload(upload)
			return nil, fmt.Errorf("failed to merge chunk %d: %w", i, err)
		}
		chunkFile.Close()
	}
	zipFile.Close()

	// 8. 生成 tokenID
	tokenID, err := tool.GetUUID()
	if err != nil {
		upload.Status = "failed"
		upload.Message = fmt.Sprintf("failed to generate tokenID: %v", err)
		s.tempAppDAO.UpdateChunkUpload(upload)
		return nil, fmt.Errorf("failed to generate tokenID: %w", err)
	}
	tokenID = strings.ReplaceAll(tokenID, "-", "_")

	// 9. 创建应用部署目录
	appDeployDir := filepath.Join(deployBaseDir, tokenID)
	if err := os.MkdirAll(appDeployDir, 0755); err != nil {
		upload.Status = "failed"
		upload.Message = fmt.Sprintf("failed to create deploy directory: %v", err)
		s.tempAppDAO.UpdateChunkUpload(upload)
		return nil, fmt.Errorf("failed to create deploy directory: %w", err)
	}

	// 10. 解压 zip 文件
	if err := s.extractZip(zipFilePath, appDeployDir); err != nil {
		os.RemoveAll(appDeployDir) // 清理目录
		upload.Status = "failed"
		upload.Message = fmt.Sprintf("failed to extract zip: %v", err)
		s.tempAppDAO.UpdateChunkUpload(upload)
		return nil, fmt.Errorf("failed to extract zip file: %w", err)
	}

	// 11. 删除 zip 文件（解压后不再需要）
	os.Remove(zipFilePath)

	// 12. 计算过期时间
	expireHours := conf.Cfg.TempApp.ExpireHours
	if expireHours == 0 {
		expireHours = 24 // 默认 24 小时
	}
	expiresAt := time.Now().Add(time.Duration(expireHours) * time.Hour)

	// 13. 创建 TempAppDeploy 记录
	deploy := &model.TempAppDeploy{
		TokenID:        tokenID,
		DeployFilePath: appDeployDir,
		ExpiresAt:      expiresAt,
		Status:         "completed",
		Message:        "",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.tempAppDAO.Create(deploy); err != nil {
		os.RemoveAll(appDeployDir) // 清理目录
		upload.Status = "failed"
		upload.Message = fmt.Sprintf("failed to create deploy record: %v", err)
		s.tempAppDAO.UpdateChunkUpload(upload)
		return nil, fmt.Errorf("failed to create deploy record: %w", err)
	}

	// 14. 更新分片上传记录
	upload.TokenID = tokenID
	upload.Status = "completed"
	upload.UpdatedAt = time.Now()
	if err := s.tempAppDAO.UpdateChunkUpload(upload); err != nil {
		// 记录错误但不影响主流程
		fmt.Printf("Failed to update chunk upload record: %v\n", err)
	}

	// 15. 删除分片文件和分片上传记录
	os.RemoveAll(chunksDir)
	s.tempAppDAO.DeleteChunkUpload(uploadID)

	return deploy, nil
}

// GetChunkUploadStatus 获取分片上传状态
// uploadID: 上传 ID
// 返回 TempAppChunkUpload 和错误
func (s *TempDeployService) GetChunkUploadStatus(uploadID string) (*model.TempAppChunkUpload, error) {
	return s.tempAppDAO.GetChunkUploadByUploadID(uploadID)
}
