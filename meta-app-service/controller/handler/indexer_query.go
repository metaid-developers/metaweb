package handler

import (
	"meta-app-service/service/indexer_service"
)

// IndexerQueryHandler indexer query handler
type IndexerQueryHandler struct {
	syncStatusService *indexer_service.SyncStatusService
}

// NewIndexerQueryHandler create indexer query handler instance
func NewIndexerQueryHandler(syncStatusService *indexer_service.SyncStatusService) *IndexerQueryHandler {
	return &IndexerQueryHandler{
		syncStatusService: syncStatusService,
	}
}
