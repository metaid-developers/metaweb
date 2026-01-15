package indexer_service

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"meta-app-service/conf"
	"meta-app-service/database"
	"meta-app-service/indexer"
	model "meta-app-service/models"
	"meta-app-service/models/dao"
	"meta-app-service/service/common_service/metaid_protocols"
	"regexp"
)

// IndexerService indexer service
type IndexerService struct {
	scanner       *indexer.BlockScanner
	syncStatusDAO *dao.IndexerSyncStatusDAO
	metaAppDAO    *dao.MetaAppDAO
	chainType     indexer.ChainType
	parser        *indexer.MetaIDParser
}

// NewIndexerService create indexer service instance
func NewIndexerService() (*IndexerService, error) {
	return NewIndexerServiceWithChain(indexer.ChainTypeMVC)
}

// NewIndexerServiceWithChain create indexer service instance with specified chain type
func NewIndexerServiceWithChain(chainType indexer.ChainType) (*IndexerService, error) {
	chainName := string(chainType)
	syncStatusDAO := dao.NewIndexerSyncStatusDAO()

	// Get current sync height from database
	var currentSyncHeight int64 = 0
	syncStatus, err := syncStatusDAO.GetByChainName(chainName)
	if err == nil && syncStatus != nil && syncStatus.CurrentSyncHeight > 0 {
		currentSyncHeight = syncStatus.CurrentSyncHeight
		log.Printf("Found existing sync status for %s chain, current sync height: %d", chainName, currentSyncHeight)
	}

	// Determine start height based on configuration
	configStartHeight := conf.Cfg.Indexer.StartHeight
	if configStartHeight == 0 {
		// Use chain-specific init height if not specified
		if chainType == indexer.ChainTypeMVC {
			configStartHeight = conf.Cfg.Indexer.MvcInitBlockHeight
		} else if chainType == indexer.ChainTypeBTC {
			configStartHeight = conf.Cfg.Indexer.BtcInitBlockHeight
		}
	}

	// Choose the higher value between config and current sync height
	startHeight := configStartHeight
	if currentSyncHeight > startHeight {
		startHeight = currentSyncHeight + 1 // Continue from next block
		log.Printf("Using current sync height + 1 as start height: %d", startHeight)
	} else if configStartHeight > 0 {
		log.Printf("Using configured start height: %d", startHeight)
	} else {
		// Default to 0 if no config and no sync status
		startHeight = 0
		log.Printf("No start height configured, starting from: %d", startHeight)
	}

	log.Printf("Indexer service will start from block height: %d (chain: %s)", startHeight, chainType)

	// Create block scanner with chain type
	scanner := indexer.NewBlockScannerWithChain(
		conf.Cfg.Chain.RpcUrl,
		conf.Cfg.Chain.RpcUser,
		conf.Cfg.Chain.RpcPass,
		startHeight,
		conf.Cfg.Indexer.ScanInterval,
		chainType,
	)

	// Enable ZMQ if configured
	if conf.Cfg.Indexer.ZmqEnabled && conf.Cfg.Indexer.ZmqAddress != "" {
		scanner.EnableZMQ(conf.Cfg.Indexer.ZmqAddress)
		log.Printf("ZMQ real-time monitoring enabled: %s", conf.Cfg.Indexer.ZmqAddress)
	} else {
		log.Println("ZMQ real-time monitoring disabled")
	}

	// Create parser
	parser := indexer.NewMetaIDParser("")
	parser.SetBlockScanner(scanner)

	service := &IndexerService{
		scanner:       scanner,
		syncStatusDAO: dao.NewIndexerSyncStatusDAO(),
		metaAppDAO:    dao.NewMetaAppDAO(),
		chainType:     chainType,
		parser:        parser,
	}

	// Initialize sync status in database
	if err := service.initializeSyncStatus(startHeight); err != nil {
		log.Printf("Failed to initialize sync status: %v", err)
	}

	return service, nil
}

// initializeSyncStatus initialize sync status in database
func (s *IndexerService) initializeSyncStatus(startHeight int64) error {
	chainName := string(s.chainType)

	// Try to get existing status
	existingStatus, err := s.syncStatusDAO.GetByChainName(chainName)
	if err == nil && existingStatus != nil {
		log.Printf("Sync status already exists for %s chain, current sync height: %d", chainName, existingStatus.CurrentSyncHeight)
		return nil
	}

	// Create initial status (only if not exists)
	initialHeight := int64(0)
	if startHeight > 0 {
		initialHeight = startHeight - 1 // Will be updated when first block is scanned
	}

	status := &model.IndexerSyncStatus{
		ChainName:         chainName,
		CurrentSyncHeight: initialHeight,
	}

	if err := s.syncStatusDAO.CreateOrUpdate(status); err != nil {
		return fmt.Errorf("failed to create sync status: %w", err)
	}

	log.Printf("Initialized sync status for %s chain with height: %d", chainName, initialHeight)
	return nil
}

// Start start indexer service
func (s *IndexerService) Start() {
	log.Println("Indexer service starting...")
	// Start deploy processor
	s.StartDeployProcessor()

	// Start block scanning with block complete callback
	s.scanner.Start(s.handleTransaction, s.onBlockComplete)

}

// GetScanner get block scanner instance
func (s *IndexerService) GetScanner() *indexer.BlockScanner {
	return s.scanner
}

// onBlockComplete called after each block is successfully scanned
func (s *IndexerService) onBlockComplete(height int64) error {
	chainName := string(s.chainType)

	// Update current sync height
	if err := s.syncStatusDAO.UpdateCurrentSyncHeight(chainName, height); err != nil {
		return fmt.Errorf("failed to update sync height: %w", err)
	}

	return nil
}

// handleTransaction handle transaction
// tx is interface{} to support both BTC (*btcwire.MsgTx) and MVC (*wire.MsgTx) transactions
func (s *IndexerService) handleTransaction(tx interface{}, metaDataTx *indexer.MetaIDDataTx, height, timestamp int64) error {
	if metaDataTx == nil || len(metaDataTx.MetaIDData) == 0 {
		return nil
	}

	// txID := metaDataTx.TxID
	// chainNameFromTx := metaDataTx.ChainName
	// pinId := metaDataTx.MetaIDData[0].PinID

	// log.Printf("Found MetaID pinId: %s,  transaction: %s at height %d (chain: %s), PIN count: %d",
	// 	pinId, txID, height, chainNameFromTx, len(metaDataTx.MetaIDData))

	// Process each PIN in the transaction
	for _, metaData := range metaDataTx.MetaIDData {
		// log.Printf("Processing PIN: %s (path: %s, operation: %s, originalPath: %s, content type: %s)",
		// 	metaData.PinID, metaData.Path, metaData.Operation, metaData.OriginalPath, metaData.ContentType)

		// Check if this is a MetaApp protocol PIN
		isMetaApp, isPathPinID := isMetaAppPath(metaData.Path)
		if isMetaApp {
			log.Printf("Processing MetaApp PIN: %s (path: %s, operation: %s, originalPath: %s)",
				metaData.PinID, metaData.Path, metaData.Operation, metaData.OriginalPath)

			// Check if already exists (by PinID)
			existingApp, err := s.metaAppDAO.GetByPinID(metaData.PinID)
			if err == nil && existingApp != nil {
				log.Printf("MetaApp PIN already indexed: %s", metaData.PinID)

				// Update block height if needed
				if existingApp.BlockHeight < height && height > 0 {
					existingApp.BlockHeight = height
					if err := s.metaAppDAO.Update(existingApp); err != nil {
						log.Printf("Failed to update MetaApp block height: %v", err)
					}
				}

				continue
			}

			if metaData.Operation == "modify" {
				log.Printf("Processing MetaApp modify operation: %s (path: %s, operation: %s, originalPath: %s)",
					metaData.PinID, metaData.Path, metaData.Operation, metaData.OriginalPath)
			}

			// 处理 modify 操作
			if metaData.Operation == "modify" && metaData.Path != "" {
				// 提取 first_pin_id 从 Path (格式: @{pin_id})，需要递归查找
				firstPinID, err := s.extractFirstPinIDFromOriginalPath(metaData.Path)
				if err != nil {
					log.Printf("Failed to extract first_pin_id from path %s: %v, skipping modify operation", metaData.Path, err)
					continue
				}

				if firstPinID != "" {
					log.Printf("Processing MetaApp modify operation: current PIN=%s, first PIN=%s",
						metaData.PinID, firstPinID)

					// 处理 modify 操作
					if err := s.processMetaAppModify(metaData, firstPinID, height, timestamp); err != nil {
						log.Printf("Failed to process MetaApp modify for PIN %s: %v", metaData.PinID, err)
						// Continue processing other PINs even if one fails
						continue
					}
					continue
				}
				continue
			}

			if metaData.Operation == "create" && isPathPinID {
				// log.Printf("Processing MetaApp create operation with path PIN: %s (path: %s, operation: %s, originalPath: %s)",
				// 	metaData.PinID, metaData.Path, metaData.Operation, metaData.OriginalPath)
				continue
			}

			// Process MetaApp content (create operation)
			if err := s.processMetaAppContent(metaData, height, timestamp); err != nil {
				log.Printf("Failed to process MetaApp content for PIN %s: %v", metaData.PinID, err)
				// Continue processing other PINs even if one fails
				continue
			}
		}
	}

	return nil
}

// isMetaAppPath check if path is a MetaApp protocol path
func isMetaAppPath(path string) (bool, isPinID bool) {
	if path == "" {
		return false, false
	}

	// 1. 检查是否匹配 MetaApp 协议路径（create 操作）
	for _, protocolPath := range metaid_protocols.ProtocolList {
		if strings.HasPrefix(path, protocolPath) || strings.Contains(path, protocolPath) {
			return true, false
		}
	}

	// 2. 检查是否是 modify 操作的 path 格式
	// modify 操作的 path 可能是：
	//   - @{pinId} - 直接引用其他 pinId
	//   - {host:@pinId} - 带 host 的格式
	if strings.HasPrefix(path, "@") {
		// 以 @ 开头，说明是 modify 操作引用其他 pinId
		return true, true
	}

	// 检查是否包含 @ 符号（可能是 {host:@pinId} 格式）
	if strings.Contains(path, "@") {
		// 提取 @ 后面的部分，检查是否符合 pinId 格式
		// 格式: 64 个十六进制字符 + 'i' + 数字
		parts := strings.Split(path, "@")
		if len(parts) > 1 {
			// 获取 @ 后面的部分
			afterAt := parts[len(parts)-1]
			// 移除可能的 } 后缀（如果是 {host:@pinId} 格式）
			afterAt = strings.TrimSuffix(afterAt, "}")
			// 检查是否符合 pinId 格式
			matched, err := regexp.MatchString(`^[0-9a-f]{64}i\d+$`, afterAt)
			if err == nil && matched {
				return true, true
			}
		}
	}

	return false, false
}

// calculateMetaID calculate MetaID from address (SHA256 hash)
func calculateMetaID(address string) string {
	if address == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(address))
	return hex.EncodeToString(hash[:])
}

// extractFirstPinIDFromOriginalPath 从 Path 中提取 first_pin_id
// Path 格式: @{pin_id}，可能是中间 pinId，需要递归查找直到找到 create 操作的 first_pin_id
func (s *IndexerService) extractFirstPinIDFromOriginalPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}

	// 移除 @ 前缀
	var currentPinID string
	if strings.HasPrefix(path, "@") {
		currentPinID = strings.TrimPrefix(path, "@")
	} else {
		currentPinID = path
	}

	if currentPinID == "" {
		return "", fmt.Errorf("invalid path format: %s", path)
	}

	// 递归查找 first_pin_id
	return s.findFirstPinIDRecursive(currentPinID, make(map[string]bool))
}

// findFirstPinIDRecursive 递归查找 first_pin_id
// visited 用于防止循环引用
func (s *IndexerService) findFirstPinIDRecursive(pinID string, visited map[string]bool) (string, error) {
	// 防止循环引用
	if visited[pinID] {
		return "", fmt.Errorf("circular reference detected for pinID: %s", pinID)
	}
	visited[pinID] = true

	// 根据 pinID 查找 MetaApp
	metaApp, err := s.metaAppDAO.GetByPinID(pinID)
	if err != nil {
		// 如果找不到，说明这个 pinID 就是 first_pin_id（可能是 create 操作还未索引）
		log.Printf("MetaApp not found for pinID %s, assuming it's first_pin_id", pinID)
		// return pinID, nil
		return "", fmt.Errorf("MetaApp not found for pinID %s", pinID)
	}

	// 如果是 create 操作，这个 pinID 就是 first_pin_id
	if metaApp.Operation == "create" {
		// 使用 FirstPinId（如果已设置），否则使用当前 PinID
		if metaApp.FirstPinId != "" {
			return metaApp.FirstPinId, nil
		}
		return metaApp.PinID, nil
	}

	// 如果是 modify 操作，需要继续向上查找
	if metaApp.Operation == "modify" {
		// 使用 FirstPinId（如果已设置）
		if metaApp.FirstPinId != "" {
			// 如果 FirstPinId 和当前 PinID 不同，继续查找
			if metaApp.FirstPinId != pinID {
				return s.findFirstPinIDRecursive(metaApp.FirstPinId, visited)
			}
			// 如果相同，说明已经找到 first_pin_id
			return metaApp.FirstPinId, nil
		}

		// 如果没有 FirstPinId，尝试从 Path 中提取（这种情况不应该发生，但作为后备）
		if metaApp.Path != "" && strings.HasPrefix(metaApp.Path, "@") {
			nextPinID := strings.TrimPrefix(metaApp.Path, "@")
			if nextPinID != "" && nextPinID != pinID {
				return s.findFirstPinIDRecursive(nextPinID, visited)
			}
		}

		// 如果无法继续查找，返回当前 PinID（作为后备）
		log.Printf("Warning: Cannot find first_pin_id for modify operation, using current pinID: %s", pinID)
		return pinID, nil
	}

	// 其他操作类型，返回当前 PinID
	return pinID, nil
}

// ensureMillisecondTimestamp 确保时间戳是 13 位（毫秒级）
func ensureMillisecondTimestamp(timestamp int64) int64 {
	// 如果时间戳是 10 位（秒级），转换为 13 位（毫秒级）
	if timestamp < 10000000000 {
		return timestamp * 1000
	}
	// 如果已经是 13 位或更长，直接返回
	return timestamp
}

// processMetaAppContent 处理并保存 MetaApp 协议内容
func (s *IndexerService) processMetaAppContent(metaData *indexer.MetaIDData, height, timestamp int64) error {
	// 获取真实的创建者地址
	creatorAddress := metaData.CreatorAddress
	if metaData.CreatorInputLocation != "" {
		realAddress, err := s.parser.FindCreatorAddressFromCreatorInputLocation(metaData.CreatorInputLocation, s.chainType)
		if err != nil {
			log.Printf("Failed to get creator address from location %s: %v, using fallback address",
				metaData.CreatorInputLocation, err)
		} else {
			creatorAddress = realAddress
			log.Printf("Found real creator address for MetaApp: %s (from location: %s)", realAddress, metaData.CreatorInputLocation)
		}
	}

	// 解析 MetaApp JSON 内容
	var metaAppProto metaid_protocols.MetaApp
	if err := json.Unmarshal(metaData.Content, &metaAppProto); err != nil {
		return fmt.Errorf("failed to parse MetaApp JSON: %w", err)
	}

	log.Printf("Parsed MetaApp: title=%s, appName=%s, version=%s, contentType=%s",
		metaAppProto.Title, metaAppProto.AppName, metaAppProto.Version, metaAppProto.ContentType)

	// 序列化 Metadata 为 JSON 字符串（如果已经是字符串则直接使用）
	metadataJSON := metaAppProto.Metadata
	if metadataJSON == "" {
		metadataJSON = "{}"
	}

	// 计算创建者 MetaID
	creatorMetaID := calculateMetaID(creatorAddress)

	// 确保时间戳是 13 位（毫秒级）
	millisecondTimestamp := ensureMillisecondTimestamp(timestamp)

	// 确定 first_pin_id（create 操作时，first_pin_id = 当前 pin_id）
	firstPinID := metaData.PinID

	// 创建数据库记录
	metaApp := &model.MetaApp{
		FirstPinId:     firstPinID,
		PinID:          metaData.PinID,
		TxID:           metaData.TxID,
		Vout:           metaData.Vout,
		Path:           metaData.Path,
		Operation:      metaData.Operation,
		ParentPath:     metaData.ParentPath,
		Title:          metaAppProto.Title,
		AppName:        metaAppProto.AppName,
		Prompt:         metaAppProto.Prompt,
		Icon:           metaAppProto.Icon,
		CoverImg:       metaAppProto.CoverImg,
		IntroImgs:      metaAppProto.IntroImgs,
		Intro:          metaAppProto.Intro,
		Runtime:        metaAppProto.Runtime,
		IndexFile:      metaAppProto.IndexFile,
		Version:        metaAppProto.Version,
		ContentType:    metaAppProto.ContentType,
		Content:        metaAppProto.Content,
		Code:           metaAppProto.Code,
		ContentHash:    metaAppProto.ContentHash,
		Metadata:       metadataJSON,
		Disabled:       metaAppProto.Disabled,
		ChainName:      metaData.ChainName,
		BlockHeight:    height,
		Timestamp:      millisecondTimestamp,
		CreatorMetaId:  creatorMetaID,
		CreatorAddress: creatorAddress,
		OwnerAddress:   metaData.OwnerAddress,
		OwnerMetaId:    calculateMetaID(metaData.OwnerAddress),
		Status:         1, // 1 表示成功
		State:          0,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// 保存到数据库
	if err := s.metaAppDAO.Create(metaApp); err != nil {
		return fmt.Errorf("failed to save MetaApp to database: %w", err)
	}

	log.Printf("MetaApp indexed successfully: PIN=%s, Title=%s, AppName=%s, Version=%s, Chain=%s",
		metaData.PinID, metaAppProto.Title, metaAppProto.AppName, metaAppProto.Version, metaData.ChainName)

	// 添加到部署队列
	if err := s.addToDeployQueue(metaApp); err != nil {
		log.Printf("Failed to add MetaApp to deploy queue: %v", err)
		// 不返回错误，因为索引已经成功
	}

	return nil
}

// processMetaAppModify 处理 MetaApp modify 操作
func (s *IndexerService) processMetaAppModify(metaData *indexer.MetaIDData, firstPinID string, height, timestamp int64) error {
	// 获取真实的创建者地址
	creatorAddress := metaData.CreatorAddress
	if metaData.CreatorInputLocation != "" {
		realAddress, err := s.parser.FindCreatorAddressFromCreatorInputLocation(metaData.CreatorInputLocation, s.chainType)
		if err != nil {
			log.Printf("Failed to get creator address from location %s: %v, using fallback address",
				metaData.CreatorInputLocation, err)
		} else {
			creatorAddress = realAddress
			log.Printf("Found real creator address for MetaApp modify: %s (from location: %s)", realAddress, metaData.CreatorInputLocation)
		}
	}

	// 解析 MetaApp JSON 内容
	var metaAppProto metaid_protocols.MetaApp
	if err := json.Unmarshal(metaData.Content, &metaAppProto); err != nil {
		return fmt.Errorf("failed to parse MetaApp JSON: %w", err)
	}

	log.Printf("Parsed MetaApp modify: title=%s, appName=%s, version=%s, contentType=%s, firstPinID=%s",
		metaAppProto.Title, metaAppProto.AppName, metaAppProto.Version, metaAppProto.ContentType, firstPinID)

	// 序列化 Metadata 为 JSON 字符串（如果已经是字符串则直接使用）
	metadataJSON := metaAppProto.Metadata
	if metadataJSON == "" {
		metadataJSON = "{}"
	}

	// 计算创建者 MetaID
	creatorMetaID := calculateMetaID(creatorAddress)

	// 确保时间戳是 13 位（毫秒级）
	millisecondTimestamp := ensureMillisecondTimestamp(timestamp)

	// 创建数据库记录（modify 操作）
	metaApp := &model.MetaApp{
		FirstPinId:     firstPinID,
		PinID:          metaData.PinID,
		TxID:           metaData.TxID,
		Vout:           metaData.Vout,
		Path:           metaData.Path,
		Operation:      metaData.Operation,
		ParentPath:     metaData.ParentPath,
		Title:          metaAppProto.Title,
		AppName:        metaAppProto.AppName,
		Prompt:         metaAppProto.Prompt,
		Icon:           metaAppProto.Icon,
		CoverImg:       metaAppProto.CoverImg,
		IntroImgs:      metaAppProto.IntroImgs,
		Intro:          metaAppProto.Intro,
		Runtime:        metaAppProto.Runtime,
		IndexFile:      metaAppProto.IndexFile,
		Version:        metaAppProto.Version,
		ContentType:    metaAppProto.ContentType,
		Content:        metaAppProto.Content,
		Code:           metaAppProto.Code,
		ContentHash:    metaAppProto.ContentHash,
		Metadata:       metadataJSON,
		Disabled:       metaAppProto.Disabled,
		ChainName:      metaData.ChainName,
		BlockHeight:    height,
		Timestamp:      millisecondTimestamp,
		CreatorMetaId:  creatorMetaID,
		CreatorAddress: creatorAddress,
		OwnerAddress:   metaData.OwnerAddress,
		OwnerMetaId:    calculateMetaID(metaData.OwnerAddress),
		Status:         1, // 1 表示成功
		State:          0,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// 保存到数据库（会更新 latest 和 history）
	if err := s.metaAppDAO.Create(metaApp); err != nil {
		return fmt.Errorf("failed to save MetaApp modify to database: %w", err)
	}

	log.Printf("MetaApp modify indexed successfully: PIN=%s, FirstPIN=%s, Title=%s, AppName=%s, Version=%s, Chain=%s",
		metaData.PinID, firstPinID, metaAppProto.Title, metaAppProto.AppName, metaAppProto.Version, metaData.ChainName)

	// 添加到部署队列
	if err := s.addToDeployQueue(metaApp); err != nil {
		log.Printf("Failed to add MetaApp modify to deploy queue: %v", err)
		// 不返回错误，因为索引已经成功
	}

	return nil
}

// addToDeployQueue 添加 MetaApp 到部署队列
func (s *IndexerService) addToDeployQueue(metaApp *model.MetaApp) error {
	if database.DB == nil {
		return fmt.Errorf("database not initialized")
	}

	// 提取 Code pinId（保持 metafile:// 格式）
	codePinID := metaApp.Code
	if codePinID == "" {
		// 如果没有 Code，尝试使用 Content（需要添加 metafile:// 前缀）
		if metaApp.Content != "" {
			if strings.HasPrefix(metaApp.Content, "metafile://") {
				codePinID = metaApp.Content
			} else {
				codePinID = "metafile://" + metaApp.Content
			}
		}
	}

	if codePinID == "" {
		log.Printf("No code or content pinId found for MetaApp %s, skipping deploy", metaApp.PinID)
		return nil
	}

	queue := &model.MetaAppDeployQueue{
		FirstPinId:  metaApp.FirstPinId,
		PinID:       metaApp.PinID,
		Timestamp:   metaApp.Timestamp,
		Content:     metaApp.Content,
		Code:        codePinID,
		ContentType: metaApp.ContentType,
		Version:     metaApp.Version,
		TryCount:    0,
		CreatedAt:   time.Now(),
	}

	return database.DB.AddToDeployQueue(queue)
}

// StartDeployProcessor 启动部署处理器（后台 goroutine）
func (s *IndexerService) StartDeployProcessor() {
	go s.deployProcessor()
	log.Println("MetaApp deploy processor started")
}

// deployProcessor 部署处理器（持续处理部署队列）
func (s *IndexerService) deployProcessor() {
	ticker := time.NewTicker(5 * time.Second) // 每 5 秒检查一次
	defer ticker.Stop()

	for range ticker.C {
		if err := s.processNextDeployItem(); err != nil {
			log.Printf("Failed to process deploy item: %v", err)
		}
	}
}

// processNextDeployItem 处理下一个部署队列项
func (s *IndexerService) processNextDeployItem() error {
	if database.DB == nil {
		return fmt.Errorf("database not initialized")
	}

	// 获取下一个待处理的队列项
	queueItem, err := database.DB.GetNextDeployQueueItem()
	if err != nil {
		if err == database.ErrNotFound {
			// 队列为空，正常情况
			return nil
		}
		return err
	}

	log.Printf("Processing deploy queue item: PinID=%s, Code=%s, TryCount=%d", queueItem.PinID, queueItem.Code, queueItem.TryCount)

	// 处理部署
	if err := s.deployMetaApp(queueItem); err != nil {
		log.Printf("Failed to deploy MetaApp %s: %v", queueItem.PinID, err)

		// 增加重试次数
		queueItem.TryCount++
		const maxRetryCount = 3

		if queueItem.TryCount >= maxRetryCount {
			// 超过最大重试次数，从队列中移除
			log.Printf("MetaApp %s exceeded max retry count (%d), removing from queue", queueItem.PinID, maxRetryCount)
			if removeErr := database.DB.RemoveFromDeployQueue(queueItem.PinID); removeErr != nil {
				log.Printf("Failed to remove from deploy queue: %v", removeErr)
			}
		} else {
			// 更新重试次数，继续保留在队列中
			if updateErr := database.DB.UpdateDeployQueueItem(queueItem); updateErr != nil {
				log.Printf("Failed to update deploy queue item: %v", updateErr)
			}
		}

		return err
	}

	// 部署成功，从队列中移除
	if err := database.DB.RemoveFromDeployQueue(queueItem.PinID); err != nil {
		log.Printf("Failed to remove from deploy queue: %v", err)
		return err
	}

	log.Printf("MetaApp deployed successfully: PinID=%s", queueItem.PinID)
	return nil
}

// deployMetaApp 部署 MetaApp（下载文件、解压、更新状态）
func (s *IndexerService) deployMetaApp(queueItem *model.MetaAppDeployQueue) error {
	// 1. 获取 MetaApp 信息
	metaApp, err := s.metaAppDAO.GetByPinID(queueItem.PinID)
	if err != nil {
		return fmt.Errorf("failed to get MetaApp: %w", err)
	}

	// 2. 创建部署目录（如果已存在且有文件，先清空）
	deployBaseDir := conf.Cfg.MetaApp.DeployFilePath
	if deployBaseDir == "" {
		deployBaseDir = "./meta_app_deploy_data"
	}
	appDeployDir := filepath.Join(deployBaseDir, metaApp.FirstPinId)

	// 检查目录是否存在
	if info, err := os.Stat(appDeployDir); err == nil && info.IsDir() {
		// 目录存在，检查是否有文件
		entries, err := os.ReadDir(appDeployDir)
		if err != nil {
			return fmt.Errorf("failed to read deploy directory: %w", err)
		}

		// 如果有文件，先清空目录
		if len(entries) > 0 {
			log.Printf("Deploy directory %s already exists with files, clearing it...", appDeployDir)
			for _, entry := range entries {
				entryPath := filepath.Join(appDeployDir, entry.Name())
				if err := os.RemoveAll(entryPath); err != nil {
					return fmt.Errorf("failed to remove existing file/directory %s: %w", entryPath, err)
				}
			}
			log.Printf("Cleared deploy directory: %s", appDeployDir)
		}
	}

	// 创建目录（如果不存在）
	if err := os.MkdirAll(appDeployDir, 0755); err != nil {
		return fmt.Errorf("failed to create deploy directory: %w", err)
	}

	// 3. 下载 Code 文件（优先使用 Code，如果没有则使用 Content）
	pinIDToDownload := queueItem.Code
	if pinIDToDownload == "" {
		// 如果没有 Code，使用 Content，并确保有 metafile:// 前缀
		if queueItem.Content != "" {
			if strings.HasPrefix(queueItem.Content, "metafile://") {
				pinIDToDownload = queueItem.Content
			} else {
				pinIDToDownload = "metafile://" + queueItem.Content
			}
		}
	}

	if pinIDToDownload == "" {
		return fmt.Errorf("no pinId to download")
	}

	// 验证 pinID 格式是否符合 metafile:// 格式
	if !isValidMetafilePinID(pinIDToDownload) {
		return fmt.Errorf("invalid pinId format: %s, expected format: metafile://<pinid>", pinIDToDownload)
	}

	// 4. 下载文件
	filePath, err := s.downloadFileFromPinID(pinIDToDownload, appDeployDir)
	if err != nil {
		log.Printf("Failed to download file from pinId: %s, error: %v", pinIDToDownload, err)
		// 下载失败，更新状态为 failed 并记录错误信息
		deployContent := &model.MetaAppDeployFileContent{
			FirstPinId:     metaApp.FirstPinId,
			PinID:          metaApp.PinID,
			Content:        queueItem.Content,
			Code:           queueItem.Code,
			ContentType:    queueItem.ContentType,
			Version:        queueItem.Version,
			DeployStatus:   "failed",
			DeployFilePath: appDeployDir,
			DeployMessage:  err.Error(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		if updateErr := database.DB.CreateOrUpdateDeployFileContent(deployContent); updateErr != nil {
			log.Printf("Failed to update deploy file content with error status: %v", updateErr)
		}

		return fmt.Errorf("failed to download file: %w", err)
	}

	// 5. 如果是 zip 文件，解压
	if strings.HasSuffix(strings.ToLower(filePath), ".zip") {
		if err := s.unzipFile(filePath, appDeployDir); err != nil {
			log.Printf("Failed to unzip file %s: %v, continuing with original file", filePath, err)
			// 不解压失败不影响部署，继续使用原文件
		} else {
			// 解压成功，删除原 zip 文件
			os.Remove(filePath)
		}
	}

	// 6. 更新部署文件内容记录
	deployContent := &model.MetaAppDeployFileContent{
		FirstPinId:     metaApp.FirstPinId,
		PinID:          metaApp.PinID,
		Content:        queueItem.Content,
		Code:           queueItem.Code,
		ContentType:    queueItem.ContentType,
		Version:        queueItem.Version,
		DeployStatus:   "completed",
		DeployFilePath: appDeployDir,
		DeployMessage:  "",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := database.DB.CreateOrUpdateDeployFileContent(deployContent); err != nil {
		return fmt.Errorf("failed to update deploy file content: %w", err)
	}
	// fmt.Printf("Deploy file content updated successfully: %+v", deployContent)

	return nil
}

// isValidMetafilePinID 验证 pinID 是否符合 metafile:// 格式
// 格式: metafile://<pinid>，其中 pinid 通常是 64 字符的十六进制字符串 + 'i' + 数字
func isValidMetafilePinID(pinID string) bool {
	// 检查是否以 metafile:// 开头
	if !strings.HasPrefix(pinID, "metafile://") {
		return false
	}

	// 提取 pinid 部分（去掉 metafile:// 前缀）
	pinIDPart := strings.TrimPrefix(pinID, "metafile://")
	if pinIDPart == "" {
		return false
	}

	// 验证 pinid 格式：通常是 64 字符的十六进制字符串 + 'i' + 数字
	// 例如: adbb39ae2b8c1129e09815d131a510268f4ba496a3d79021a8c4dc78f4dbb875i0
	matched, err := regexp.MatchString(`^[0-9a-f]{64}i\d+$`, pinIDPart)
	if err != nil {
		return false
	}

	return matched
}

// downloadFileFromPinID 从 pinId 下载文件
func (s *IndexerService) downloadFileFromPinID(pinID, targetDir string) (string, error) {
	// 验证 pinID 格式
	if !isValidMetafilePinID(pinID) {
		return "", fmt.Errorf("invalid pinId format: %s, expected format: metafile://<pinid>", pinID)
	}

	// 提取实际的 pinid（去掉 metafile:// 前缀）
	actualPinID := strings.TrimPrefix(pinID, "metafile://")

	// 使用 metafs 服务下载文件
	if conf.Cfg.Metafs.Domain == "" {
		return "", fmt.Errorf("metafs domain not configured")
	}
	return s.downloadFileFromMetafs(actualPinID, targetDir)
}

// MetafsResponse Metafs 统一响应结构
type MetafsResponse struct {
	Code           int             `json:"code"`
	Message        string          `json:"message"`
	ProcessingTime int             `json:"processingTime"`
	Data           *MetafsFileInfo `json:"data"`
}

// MetafsFileInfo Metafs 文件信息结构
type MetafsFileInfo struct {
	PinID          string `json:"pin_id"`
	TxID           string `json:"tx_id"`
	Path           string `json:"path"`
	Operation      string `json:"operation"`
	Encryption     string `json:"encryption"`
	ContentType    string `json:"content_type"`
	FileType       string `json:"file_type"`
	FileExtension  string `json:"file_extension"`
	FileName       string `json:"file_name"`
	FileSize       int64  `json:"file_size"`
	FileMd5        string `json:"file_md5"`
	FileHash       string `json:"file_hash"`
	StoragePath    string `json:"storage_path"`
	ChainName      string `json:"chain_name"`
	BlockHeight    int64  `json:"block_height"`
	Timestamp      int64  `json:"timestamp"`
	CreatorMetaId  string `json:"creator_meta_id"`
	CreatorAddress string `json:"creator_address"`
	OwnerMetaId    string `json:"owner_meta_id"`
	OwnerAddress   string `json:"owner_address"`
}

// downloadFileFromMetafs 从 metafs 服务下载文件
func (s *IndexerService) downloadFileFromMetafs(pinID, targetDir string) (string, error) {
	domain := conf.Cfg.Metafs.Domain
	if domain == "" {
		return "", fmt.Errorf("metafs domain not configured")
	}

	// 1. 先获取文件信息，检查文件是否存在
	fileInfoURL := fmt.Sprintf("%s/api/v1/files/%s", strings.TrimSuffix(domain, "/"), pinID)
	log.Printf("Fetching file info from metafs: %s", fileInfoURL)

	resp, err := http.Get(fileInfoURL)
	if err != nil {
		return "", fmt.Errorf("failed to get file info from metafs: %w", err)
	}
	defer resp.Body.Close()

	var metafsResp MetafsResponse
	if err := json.NewDecoder(resp.Body).Decode(&metafsResp); err != nil {
		return "", fmt.Errorf("failed to decode file info response: %w", err)
	}

	// 2. 检查文件是否存在
	if metafsResp.Code != 0 || metafsResp.Data == nil {
		return "", fmt.Errorf("file not found in metafs: %s (code: %d, message: %s)", pinID, metafsResp.Code, metafsResp.Message)
	}

	fileInfo := metafsResp.Data

	// 3. 使用文件信息确定文件扩展名和文件名
	fileExt := fileInfo.FileExtension
	if fileExt == "" {
		fileExt = getFileExtensionFromContentType(fileInfo.ContentType)
		if fileExt == "" {
			fileExt = ".bin"
		}
	}

	// 4. 判断是否为 HTML 文件，如果是则直接使用 index.html 作为文件名
	var fileName string
	if strings.ToLower(fileExt) == ".html" || strings.ToLower(fileExt) == ".htm" ||
		strings.Contains(strings.ToLower(fileInfo.ContentType), "html") {
		fileName = "index.html"
	} else {
		// 非 HTML 文件，使用原始文件名或 pinID + 扩展名
		fileName = fileInfo.FileName
		if fileName == "" {
			fileName = pinID + fileExt
		}
	}

	// 5. 下载文件内容
	downloadURL := fmt.Sprintf("%s/api/v1/files/accelerate/content/%s", strings.TrimSuffix(domain, "/"), pinID)
	log.Printf("Downloading file from metafs: %s", downloadURL)

	downloadResp, err := http.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download file from metafs: %w", err)
	}
	defer downloadResp.Body.Close()

	if downloadResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("metafs returned status %d for file download", downloadResp.StatusCode)
	}

	// 6. 保存文件
	filePath := filepath.Join(targetDir, fileName)
	outFile, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	written, err := io.Copy(outFile, downloadResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	log.Printf("Downloaded file from metafs: %s (size: %d bytes, expected: %d bytes)", filePath, written, fileInfo.FileSize)

	return filePath, nil
}

// getFileExtensionFromContentType 根据内容类型获取文件扩展名
func getFileExtensionFromContentType(contentType string) string {
	contentType = strings.ToLower(contentType)
	if strings.Contains(contentType, "zip") {
		return ".zip"
	}
	if strings.Contains(contentType, "javascript") || strings.Contains(contentType, "ecmascript") {
		return ".js"
	}
	if strings.Contains(contentType, "html") {
		return ".html"
	}
	if strings.Contains(contentType, "css") {
		return ".css"
	}
	if strings.Contains(contentType, "json") {
		return ".json"
	}
	return ""
}

// unzipFile 解压 zip 文件
func (s *IndexerService) unzipFile(zipPath, targetDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// 安全检查：防止路径遍历攻击
		fpath := filepath.Join(targetDir, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(targetDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	log.Printf("Unzipped file: %s to %s", zipPath, targetDir)
	return nil
}
