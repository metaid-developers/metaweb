package database

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	model "meta-app-service/models"

	"github.com/cockroachdb/pebble"
)

// PebbleDatabase PebbleDB database implementation with multiple collections
type PebbleDatabase struct {
	collections map[string]*pebble.DB // Map of collection name to PebbleDB instance

	statusIDCounter atomic.Int64
}

// PebbleConfig PebbleDB configuration
type PebbleConfig struct {
	DataDir string
}

// Collection names and their key-value formats
const (
	// MetaApp collections
	collectionMetaAppPinID           = "metaapp_pin"            // key: {pin_id}, value: JSON(MetaApp) - PinID 到 MetaApp 的映射
	collectionMetaAppPinIDLastest    = "metaapp_pin_latest"     // key: {first_pin_id}, value: JSON(MetaApp) - 最新 MetaApp
	collectionMetaAppPinIDHistory    = "metaapp_pin_history"    // key: {first_pin_id}, value:  JSON(MetaApp) list - 历史 MetaApp
	collectionMetaAppMetaIDTimestamp = "metaapp_meta_timestamp" // key: {meta_id}:{timestamp}:{first_pin_id}, value: JSON(MetaApp) - 按 MetaID 和时间戳索引
	collectionMetaAppTimestamp       = "metaapp_timestamp"      // key: {timestamp}:{first_pin_id}, value: JSON(MetaApp) - 按时间戳索引（用于全局列表）

	collectionMetaAppDeployFileContent = "metaapp_deploy_file_content" // key: {pin_id}, value: JSON(MetaAppDeployFileContent) - 部署文件内容
	collectionMetaAppDeployQueue       = "metaapp_deploy_queue"        // key: {reverse_timestamp}:{pin_id}, value: JSON(MetaAppDeployQueue) - 部署队列（按时间戳倒序）

	collectionTempAppDeploy      = "temp_app_deploy"       // key: {token_id}, value: JSON(TempAppDeploy) - 临时应用部署
	collectionTempAppChunkUpload = "temp_app_chunk_upload" // key: {upload_id}, value: JSON(TempAppChunkUpload) - 临时应用分片上传

	// System collections
	collectionSyncStatus = "sync_status" // key: {chain_name}, value: JSON(IndexerSyncStatus) - 同步状态
	collectionCounters   = "counters"    // key: status, value: {max_id} - ID 计数器
)

// Counter keys
const (
	keyStatusCounter = "status"
)

// NewPebbleDatabase create PebbleDB database instance with multiple collections
func NewPebbleDatabase(config interface{}) (Database, error) {
	cfg, ok := config.(*PebbleConfig)
	if !ok {
		return nil, fmt.Errorf("invalid PebbleDB config type")
	}

	// Create data directory if not exists with full permissions
	if err := os.MkdirAll(cfg.DataDir, 0777); err != nil {
		return nil, fmt.Errorf("failed to create data directory %s: %w", cfg.DataDir, err)
	}

	log.Printf("PebbleDB data directory: %s", cfg.DataDir)

	// List of all collections
	collectionNames := []string{
		collectionMetaAppPinID,
		collectionMetaAppPinIDLastest,
		collectionMetaAppPinIDHistory,
		collectionMetaAppMetaIDTimestamp,
		collectionMetaAppTimestamp,
		collectionMetaAppDeployFileContent,
		collectionMetaAppDeployQueue,
		collectionTempAppDeploy,
		collectionTempAppChunkUpload,
		collectionSyncStatus,
		collectionCounters,
	}

	// Open PebbleDB for each collection
	collections := make(map[string]*pebble.DB)
	for _, name := range collectionNames {
		// Create collection path: dataDir/collectionName
		collectionPath := filepath.Join(cfg.DataDir, "indexer_db", name)

		log.Printf("Opening collection: %s at %s", name, collectionPath)

		// PebbleDB will create the directory automatically, but we ensure parent exists
		// No need to create the collection directory manually
		db, err := pebble.Open(collectionPath, &pebble.Options{})
		if err != nil {
			// Close previously opened databases
			for _, openedDB := range collections {
				openedDB.Close()
			}
			return nil, fmt.Errorf("failed to open collection %s at %s: %w", name, collectionPath, err)
		}
		collections[name] = db
		log.Printf("Collection %s opened successfully", name)
	}

	pdb := &PebbleDatabase{
		collections: collections,
	}

	// Load counters
	if err := pdb.loadCounters(); err != nil {
		return nil, fmt.Errorf("failed to load counters: %w", err)
	}

	log.Printf("PebbleDB database connected successfully with %d collections", len(collections))
	return pdb, nil
}

// loadCounters load ID counters from counters collection
func (p *PebbleDatabase) loadCounters() error {
	counterDB := p.collections[collectionCounters]

	// Load status counter
	if val, closer, err := counterDB.Get([]byte(keyStatusCounter)); err == nil {
		count, _ := strconv.ParseInt(string(val), 10, 64)
		p.statusIDCounter.Store(count)
		closer.Close()
	}

	return nil
}

// MetaApp operations

// paginateMetaAppsByTimestampDesc sorts MetaApps by timestamp desc (fallback PinID) then slices by cursor+size.
func paginateMetaAppsByTimestampDesc(apps []*model.MetaApp, cursor int64, size int) ([]*model.MetaApp, int64) {
	if len(apps) == 0 || size <= 0 {
		return nil, cursor
	}

	if cursor < 0 {
		cursor = 0
	}

	sort.Slice(apps, func(i, j int) bool {
		if apps[i].Timestamp == apps[j].Timestamp {
			return apps[i].PinID > apps[j].PinID
		}
		return apps[i].Timestamp > apps[j].Timestamp
	})

	start := int(cursor)
	if start >= len(apps) {
		return nil, cursor
	}

	end := start + size
	if end > len(apps) {
		end = len(apps)
	}

	paged := apps[start:end]
	nextCursor := cursor + int64(len(paged))
	return paged, nextCursor
}

func (p *PebbleDatabase) CreateMetaApp(app *model.MetaApp) error {
	// Serialize MetaApp
	data, err := json.Marshal(app)
	if err != nil {
		return err
	}

	// 确保 FirstPinId 已设置（如果为空，使用当前 PinID）
	firstPinID := app.FirstPinId
	if firstPinID == "" {
		firstPinID = app.PinID
		app.FirstPinId = firstPinID
		// 重新序列化以包含 FirstPinId
		data, err = json.Marshal(app)
		if err != nil {
			return err
		}
	}

	// Store in PinID collection (primary index)
	// key: pin_id, value: JSON(MetaApp)
	if err := p.collections[collectionMetaAppPinID].Set([]byte(app.PinID), data, pebble.Sync); err != nil {
		return err
	}

	// Store in Latest collection
	// key: first_pin_id, value: JSON(MetaApp) - 最新的 MetaApp
	if err := p.collections[collectionMetaAppPinIDLastest].Set([]byte(firstPinID), data, pebble.Sync); err != nil {
		return err
	}

	// Store in History collection
	// key: first_pin_id, value: JSON array of MetaApp - 历史列表
	if err := p.addToHistory(firstPinID, app); err != nil {
		return err
	}

	// Store in MetaID+Timestamp index collection
	// key: meta_id:reverse_timestamp:first_pin_id, value: JSON(MetaApp)
	// Format: {meta_id}:{reverse_timestamp}:{first_pin_id} for sorting by timestamp desc
	// Use reverse timestamp (max_int64 - timestamp) for descending order
	// 注意：这里需要删除旧的索引（如果有的话），因为 first_pin_id 可能相同但 timestamp 不同
	reverseTimestamp := int64(^uint64(0)>>1) - app.Timestamp
	reverseTimestampKey := strconv.FormatInt(reverseTimestamp, 10)
	metaIDTimestampKey := app.CreatorMetaId + ":" + reverseTimestampKey + ":" + firstPinID

	// 删除旧的索引（如果有相同 first_pin_id 但不同 timestamp 的旧记录）
	// 通过遍历找到旧的索引并删除
	prefix := app.CreatorMetaId + ":"
	iter, err := p.collections[collectionMetaAppMetaIDTimestamp].NewIter(&pebble.IterOptions{
		LowerBound: []byte(prefix),
		UpperBound: []byte(prefix + "~"),
	})
	if err == nil {
		for iter.First(); iter.Valid(); iter.Next() {
			key := string(iter.Key())
			// 检查是否是同一个 first_pin_id 的旧记录
			if strings.HasSuffix(key, ":"+firstPinID) && key != metaIDTimestampKey {
				// 删除旧的索引
				p.collections[collectionMetaAppMetaIDTimestamp].Delete(iter.Key(), pebble.Sync)
			}
		}
		iter.Close()
	}

	if err := p.collections[collectionMetaAppMetaIDTimestamp].Set([]byte(metaIDTimestampKey), data, pebble.Sync); err != nil {
		return err
	}

	// Store in Timestamp index collection (for global list)
	// key: reverse_timestamp:first_pin_id, value: JSON(MetaApp)
	// Use reverse timestamp for descending order
	// 同样需要删除旧的索引
	timestampIndexKey := reverseTimestampKey + ":" + firstPinID

	// 删除旧的全局索引
	globalIter, err := p.collections[collectionMetaAppTimestamp].NewIter(nil)
	if err == nil {
		for globalIter.First(); globalIter.Valid(); globalIter.Next() {
			key := string(globalIter.Key())
			// 检查是否是同一个 first_pin_id 的旧记录
			if strings.HasSuffix(key, ":"+firstPinID) && key != timestampIndexKey {
				// 删除旧的索引
				p.collections[collectionMetaAppTimestamp].Delete(globalIter.Key(), pebble.Sync)
			}
		}
		globalIter.Close()
	}

	if err := p.collections[collectionMetaAppTimestamp].Set([]byte(timestampIndexKey), data, pebble.Sync); err != nil {
		return err
	}

	return nil
}

// addToHistory 添加 MetaApp 到历史记录
func (p *PebbleDatabase) addToHistory(firstPinID string, app *model.MetaApp) error {
	historyDB := p.collections[collectionMetaAppPinIDHistory]

	// 获取现有历史记录
	var history []*model.MetaApp
	if data, closer, err := historyDB.Get([]byte(firstPinID)); err == nil {
		if err := json.Unmarshal(data, &history); err == nil {
			// 历史记录存在，添加新的记录
		}
		closer.Close()
	}

	// 添加新记录到历史
	history = append(history, app)

	// 按时间戳排序（最新的在前）
	sort.Slice(history, func(i, j int) bool {
		return history[i].Timestamp > history[j].Timestamp
	})

	// 序列化历史记录
	historyData, err := json.Marshal(history)
	if err != nil {
		return err
	}

	// 保存历史记录
	return historyDB.Set([]byte(firstPinID), historyData, pebble.Sync)
}

func (p *PebbleDatabase) GetMetaAppByPinID(pinID string) (*model.MetaApp, error) {
	// Get MetaApp data directly from PinID collection
	data, closer, err := p.collections[collectionMetaAppPinID].Get([]byte(pinID))
	if err != nil {
		if err == pebble.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer closer.Close()

	var app model.MetaApp
	if err := json.Unmarshal(data, &app); err != nil {
		return nil, err
	}

	return &app, nil
}

func (p *PebbleDatabase) UpdateMetaApp(app *model.MetaApp) error {
	// Simply recreate (overwrite)
	return p.CreateMetaApp(app)
}

func (p *PebbleDatabase) GetMetaAppsByCreatorMetaIDWithCursor(metaID string, cursor int64, size int) ([]*model.MetaApp, int64, error) {
	metaIDTimestampDB := p.collections[collectionMetaAppMetaIDTimestamp]
	prefix := metaID + ":"

	// Create iterator with prefix
	// key format: meta_id:reverse_timestamp:first_pin_id
	iter, err := metaIDTimestampDB.NewIter(&pebble.IterOptions{
		LowerBound: []byte(prefix),
		UpperBound: []byte(prefix + "~"),
	})
	if err != nil {
		return nil, 0, err
	}
	defer iter.Close()

	// 使用 map 去重，确保每个 first_pin_id 只保留最新的（由于索引已按 reverse_timestamp 排序，第一个就是最新的）
	firstPinIDMap := make(map[string]*model.MetaApp)
	for iter.First(); iter.Valid(); iter.Next() {
		var app model.MetaApp
		if err := json.Unmarshal(iter.Value(), &app); err != nil {
			continue
		}

		// 确保 FirstPinId 已设置
		firstPinID := app.FirstPinId
		if firstPinID == "" {
			firstPinID = app.PinID
		}

		// 如果这个 first_pin_id 还没有记录，或者当前记录的时间戳更新，则更新
		if existing, exists := firstPinIDMap[firstPinID]; !exists || app.Timestamp > existing.Timestamp {
			firstPinIDMap[firstPinID] = &app
		}
	}

	// 转换为列表并排序
	apps := make([]*model.MetaApp, 0, len(firstPinIDMap))
	for _, app := range firstPinIDMap {
		apps = append(apps, app)
	}

	// Apps are already sorted by reverse timestamp (descending), but we need to sort by actual timestamp desc
	sorted, nextCursor := paginateMetaAppsByTimestampDesc(apps, cursor, size)
	return sorted, nextCursor, nil
}

func (p *PebbleDatabase) ListMetaAppsWithCursor(cursor int64, size int) ([]*model.MetaApp, int64, error) {
	timestampDB := p.collections[collectionMetaAppTimestamp]

	// Create iterator for timestamp collection
	// key format: reverse_timestamp:first_pin_id
	iter, err := timestampDB.NewIter(nil)
	if err != nil {
		return nil, 0, err
	}
	defer iter.Close()

	// 使用 map 去重，确保每个 first_pin_id 只保留最新的（由于索引已按 reverse_timestamp 排序，第一个就是最新的）
	firstPinIDMap := make(map[string]*model.MetaApp)
	for iter.First(); iter.Valid(); iter.Next() {
		var app model.MetaApp
		if err := json.Unmarshal(iter.Value(), &app); err != nil {
			continue
		}

		// 确保 FirstPinId 已设置
		firstPinID := app.FirstPinId
		if firstPinID == "" {
			firstPinID = app.PinID
		}

		// 如果这个 first_pin_id 还没有记录，或者当前记录的时间戳更新，则更新
		if existing, exists := firstPinIDMap[firstPinID]; !exists || app.Timestamp > existing.Timestamp {
			firstPinIDMap[firstPinID] = &app
		}
	}

	// 转换为列表并排序
	apps := make([]*model.MetaApp, 0, len(firstPinIDMap))
	for _, app := range firstPinIDMap {
		apps = append(apps, app)
	}

	// Apps are already sorted by reverse timestamp (descending), but we need to sort by actual timestamp desc
	sorted, nextCursor := paginateMetaAppsByTimestampDesc(apps, cursor, size)
	return sorted, nextCursor, nil
}

func (p *PebbleDatabase) CountMetaApps() (int64, error) {
	// 统计唯一的 first_pin_id 数量（从 latest collection）
	latestDB := p.collections[collectionMetaAppPinIDLastest]

	// Create iterator to count all unique first_pin_id
	iter, err := latestDB.NewIter(nil)
	if err != nil {
		return 0, err
	}
	defer iter.Close()

	var count int64
	for iter.First(); iter.Valid(); iter.Next() {
		count++
	}

	return count, nil
}

// GetLatestMetaAppByFirstPinID 根据 first_pin_id 获取最新的 MetaApp
func (p *PebbleDatabase) GetLatestMetaAppByFirstPinID(firstPinID string) (*model.MetaApp, error) {
	latestDB := p.collections[collectionMetaAppPinIDLastest]

	data, closer, err := latestDB.Get([]byte(firstPinID))
	if err != nil {
		if err == pebble.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer closer.Close()

	var app model.MetaApp
	if err := json.Unmarshal(data, &app); err != nil {
		return nil, err
	}

	return &app, nil
}

// GetMetaAppHistoryByFirstPinID 根据 first_pin_id 获取历史记录
func (p *PebbleDatabase) GetMetaAppHistoryByFirstPinID(firstPinID string) ([]*model.MetaApp, error) {
	historyDB := p.collections[collectionMetaAppPinIDHistory]

	data, closer, err := historyDB.Get([]byte(firstPinID))
	if err != nil {
		if err == pebble.ErrNotFound {
			return []*model.MetaApp{}, nil // 返回空列表而不是错误
		}
		return nil, err
	}
	defer closer.Close()

	var history []*model.MetaApp
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}

	return history, nil
}

// IndexerSyncStatus operations

func (p *PebbleDatabase) CreateOrUpdateIndexerSyncStatus(status *model.IndexerSyncStatus) error {
	if status.ID == 0 {
		status.ID = p.statusIDCounter.Add(1)
		// Save counter
		p.collections[collectionCounters].Set(
			[]byte(keyStatusCounter),
			[]byte(strconv.FormatInt(status.ID, 10)),
			pebble.Sync,
		)
	}

	data, err := json.Marshal(status)
	if err != nil {
		return err
	}

	syncStatusDB := p.collections[collectionSyncStatus]

	// Store by chain name (primary key for sync status)
	return syncStatusDB.Set([]byte(status.ChainName), data, pebble.Sync)
}

func (p *PebbleDatabase) GetIndexerSyncStatusByChainName(chainName string) (*model.IndexerSyncStatus, error) {
	syncStatusDB := p.collections[collectionSyncStatus]

	data, closer, err := syncStatusDB.Get([]byte(chainName))
	if err != nil {
		if err == pebble.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer closer.Close()

	var status model.IndexerSyncStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, err
	}

	return &status, nil
}

func (p *PebbleDatabase) UpdateIndexerSyncStatusHeight(chainName string, height int64) error {
	status, err := p.GetIndexerSyncStatusByChainName(chainName)
	if err != nil {
		return err
	}

	status.CurrentSyncHeight = height
	return p.CreateOrUpdateIndexerSyncStatus(status)
}

func (p *PebbleDatabase) GetAllIndexerSyncStatus() ([]*model.IndexerSyncStatus, error) {
	var statuses []*model.IndexerSyncStatus

	syncStatusDB := p.collections[collectionSyncStatus]

	iter, err := syncStatusDB.NewIter(nil)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		var status model.IndexerSyncStatus
		if err := json.Unmarshal(iter.Value(), &status); err != nil {
			continue
		}
		statuses = append(statuses, &status)
	}

	return statuses, nil
}

// MetaApp deploy operations

// AddToDeployQueue 添加 MetaApp 到部署队列
func (p *PebbleDatabase) AddToDeployQueue(queue *model.MetaAppDeployQueue) error {
	data, err := json.Marshal(queue)
	if err != nil {
		return err
	}

	// key: reverse_timestamp:pin_id (用于按时间倒序排列)
	reverseTimestamp := int64(^uint64(0)>>1) - queue.Timestamp
	reverseTimestampKey := strconv.FormatInt(reverseTimestamp, 10)
	queueKey := reverseTimestampKey + ":" + queue.PinID

	return p.collections[collectionMetaAppDeployQueue].Set([]byte(queueKey), data, pebble.Sync)
}

// GetDeployQueueItem 获取部署队列项
func (p *PebbleDatabase) GetDeployQueueItem(pinID string) (*model.MetaAppDeployQueue, error) {
	queueDB := p.collections[collectionMetaAppDeployQueue]

	// 遍历查找匹配的 pinID
	iter, err := queueDB.NewIter(nil)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		var queue model.MetaAppDeployQueue
		if err := json.Unmarshal(iter.Value(), &queue); err != nil {
			continue
		}
		if queue.PinID == pinID {
			return &queue, nil
		}
	}

	return nil, ErrNotFound
}

// UpdateDeployQueueItem 更新部署队列项
func (p *PebbleDatabase) UpdateDeployQueueItem(queue *model.MetaAppDeployQueue) error {
	queueDB := p.collections[collectionMetaAppDeployQueue]

	// 遍历查找匹配的 pinID
	iter, err := queueDB.NewIter(nil)
	if err != nil {
		return err
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		var existingQueue model.MetaAppDeployQueue
		if err := json.Unmarshal(iter.Value(), &existingQueue); err != nil {
			continue
		}
		if existingQueue.PinID == queue.PinID {
			// 找到匹配的项，更新它
			data, err := json.Marshal(queue)
			if err != nil {
				return err
			}
			return queueDB.Set(iter.Key(), data, pebble.Sync)
		}
	}

	return ErrNotFound
}

// RemoveFromDeployQueue 从部署队列中移除
func (p *PebbleDatabase) RemoveFromDeployQueue(pinID string) error {
	queueDB := p.collections[collectionMetaAppDeployQueue]

	// 遍历查找并删除匹配的 pinID
	iter, err := queueDB.NewIter(nil)
	if err != nil {
		return err
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		var queue model.MetaAppDeployQueue
		if err := json.Unmarshal(iter.Value(), &queue); err != nil {
			continue
		}
		if queue.PinID == pinID {
			return queueDB.Delete(iter.Key(), pebble.Sync)
		}
	}

	return ErrNotFound
}

// GetNextDeployQueueItem 获取下一个待处理的部署队列项（按时间戳倒序，最早的优先）
func (p *PebbleDatabase) GetNextDeployQueueItem() (*model.MetaAppDeployQueue, error) {
	queueDB := p.collections[collectionMetaAppDeployQueue]

	// 创建迭代器（按 reverse_timestamp 排序，所以第一个是最早的）
	iter, err := queueDB.NewIter(nil)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	if !iter.First() {
		return nil, ErrNotFound
	}

	var queue model.MetaAppDeployQueue
	if err := json.Unmarshal(iter.Value(), &queue); err != nil {
		return nil, err
	}

	return &queue, nil
}

// ListDeployQueueWithCursor 获取部署队列列表（支持游标分页，按时间戳倒序）
func (p *PebbleDatabase) ListDeployQueueWithCursor(cursor int64, size int) ([]*model.MetaAppDeployQueue, int64, error) {
	queueDB := p.collections[collectionMetaAppDeployQueue]

	// 创建迭代器（按 reverse_timestamp 排序，所以第一个是最早的）
	// key format: reverse_timestamp:pin_id
	iter, err := queueDB.NewIter(nil)
	if err != nil {
		return nil, 0, err
	}
	defer iter.Close()

	// 收集所有队列项
	queues := make([]*model.MetaAppDeployQueue, 0)
	for iter.First(); iter.Valid(); iter.Next() {
		var queue model.MetaAppDeployQueue
		if err := json.Unmarshal(iter.Value(), &queue); err != nil {
			continue
		}
		queues = append(queues, &queue)
	}

	// 按时间戳倒序排序（最新的在前）
	sort.Slice(queues, func(i, j int) bool {
		if queues[i].Timestamp == queues[j].Timestamp {
			return queues[i].PinID > queues[j].PinID
		}
		return queues[i].Timestamp > queues[j].Timestamp
	})

	// 分页处理
	if cursor < 0 {
		cursor = 0
	}
	if size <= 0 {
		size = 20
	}

	start := int(cursor)
	if start >= len(queues) {
		return []*model.MetaAppDeployQueue{}, cursor, nil
	}

	end := start + size
	if end > len(queues) {
		end = len(queues)
	}

	paged := queues[start:end]
	nextCursor := cursor + int64(len(paged))
	return paged, nextCursor, nil
}

// CreateOrUpdateDeployFileContent 创建或更新部署文件内容
func (p *PebbleDatabase) CreateOrUpdateDeployFileContent(content *model.MetaAppDeployFileContent) error {
	data, err := json.Marshal(content)
	if err != nil {
		return err
	}

	// key: pin_id
	return p.collections[collectionMetaAppDeployFileContent].Set([]byte(content.PinID), data, pebble.Sync)
}

// GetDeployFileContent 获取部署文件内容
func (p *PebbleDatabase) GetDeployFileContent(pinID string) (*model.MetaAppDeployFileContent, error) {
	contentDB := p.collections[collectionMetaAppDeployFileContent]

	data, closer, err := contentDB.Get([]byte(pinID))
	if err != nil {
		if err == pebble.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer closer.Close()

	var content model.MetaAppDeployFileContent
	if err := json.Unmarshal(data, &content); err != nil {
		return nil, err
	}

	return &content, nil
}

// TempApp deploy operations

// CreateTempAppDeploy 创建临时应用部署记录
func (p *PebbleDatabase) CreateTempAppDeploy(deploy *model.TempAppDeploy) error {
	data, err := json.Marshal(deploy)
	if err != nil {
		return err
	}

	// key: token_id
	return p.collections[collectionTempAppDeploy].Set([]byte(deploy.TokenID), data, pebble.Sync)
}

// GetTempAppDeployByTokenID 根据 TokenID 获取临时应用部署记录
func (p *PebbleDatabase) GetTempAppDeployByTokenID(tokenID string) (*model.TempAppDeploy, error) {
	deployDB := p.collections[collectionTempAppDeploy]

	data, closer, err := deployDB.Get([]byte(tokenID))
	if err != nil {
		if err == pebble.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer closer.Close()

	var deploy model.TempAppDeploy
	if err := json.Unmarshal(data, &deploy); err != nil {
		return nil, err
	}

	return &deploy, nil
}

// DeleteTempAppDeploy 删除临时应用部署记录
func (p *PebbleDatabase) DeleteTempAppDeploy(tokenID string) error {
	deployDB := p.collections[collectionTempAppDeploy]
	return deployDB.Delete([]byte(tokenID), pebble.Sync)
}

// ListExpiredTempAppDeploys 获取所有过期的临时应用部署记录
func (p *PebbleDatabase) ListExpiredTempAppDeploys() ([]*model.TempAppDeploy, error) {
	deployDB := p.collections[collectionTempAppDeploy]

	iter, err := deployDB.NewIter(nil)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	now := time.Now()
	expired := make([]*model.TempAppDeploy, 0)

	for iter.First(); iter.Valid(); iter.Next() {
		var deploy model.TempAppDeploy
		if err := json.Unmarshal(iter.Value(), &deploy); err != nil {
			continue
		}

		// 检查是否过期
		if deploy.ExpiresAt.Before(now) {
			expired = append(expired, &deploy)
		}
	}

	return expired, nil
}

// TempApp chunk upload operations

// CreateTempAppChunkUpload 创建临时应用分片上传记录
func (p *PebbleDatabase) CreateTempAppChunkUpload(upload *model.TempAppChunkUpload) error {
	data, err := json.Marshal(upload)
	if err != nil {
		return err
	}

	// key: upload_id
	return p.collections[collectionTempAppChunkUpload].Set([]byte(upload.UploadID), data, pebble.Sync)
}

// GetTempAppChunkUploadByUploadID 根据 UploadID 获取临时应用分片上传记录
func (p *PebbleDatabase) GetTempAppChunkUploadByUploadID(uploadID string) (*model.TempAppChunkUpload, error) {
	uploadDB := p.collections[collectionTempAppChunkUpload]

	data, closer, err := uploadDB.Get([]byte(uploadID))
	if err != nil {
		if err == pebble.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer closer.Close()

	var upload model.TempAppChunkUpload
	if err := json.Unmarshal(data, &upload); err != nil {
		return nil, err
	}

	return &upload, nil
}

// UpdateTempAppChunkUpload 更新临时应用分片上传记录
func (p *PebbleDatabase) UpdateTempAppChunkUpload(upload *model.TempAppChunkUpload) error {
	data, err := json.Marshal(upload)
	if err != nil {
		return err
	}

	// key: upload_id
	return p.collections[collectionTempAppChunkUpload].Set([]byte(upload.UploadID), data, pebble.Sync)
}

// DeleteTempAppChunkUpload 删除临时应用分片上传记录
func (p *PebbleDatabase) DeleteTempAppChunkUpload(uploadID string) error {
	uploadDB := p.collections[collectionTempAppChunkUpload]
	return uploadDB.Delete([]byte(uploadID), pebble.Sync)
}

// Close close all database connections
func (p *PebbleDatabase) Close() error {
	var lastErr error
	for name, db := range p.collections {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close collection %s: %v", name, err)
			lastErr = err
		}
	}
	return lastErr
}
