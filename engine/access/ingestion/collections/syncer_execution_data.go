package collections

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
)

// ExecutionDataSyncer submits collections from execution data to the collections indexer. It is
// designed to be used within the collection syncer to optimize indexing when collection data is
// already available on the node.
type ExecutionDataSyncer struct {
	executionDataCache execution_data.ExecutionDataCache
	indexer            CollectionIndexer
}

func NewExecutionDataSyncer(
	executionDataCache execution_data.ExecutionDataCache,
	indexer CollectionIndexer,
) *ExecutionDataSyncer {
	return &ExecutionDataSyncer{
		executionDataCache: executionDataCache,
		indexer:            indexer,
	}
}

// IndexForHeight indexes the collections for a given height using locally available execution data.
// Returns false and no error if execution data for the block is not available.
//
// No error returns are expected during normal operation.
func (s *ExecutionDataSyncer) IndexForHeight(ctx context.Context, height uint64) (bool, error) {
	executionData, err := s.executionDataCache.ByHeight(ctx, height)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) || execution_data.IsBlobNotFoundError(err) {
			return false, nil // data for the block is not available yet.
		}
		return false, fmt.Errorf("failed to get execution data for height %d: %w", height, err)
	}

	// index all collections except for the system chunk.
	for _, chunkData := range executionData.ChunkExecutionDatas[:len(executionData.ChunkExecutionDatas)-1] {
		s.indexer.OnCollectionReceived(chunkData.Collection)
	}

	return true, nil
}

// IndexFromStartHeight indexes the collections for all blocks with available execution data starting
// from the last full block height. Returns the last indexed height.
//
// No error returns are expected during normal operation.
func (s *ExecutionDataSyncer) IndexFromStartHeight(ctx context.Context, lastFullBlockHeight uint64) (uint64, error) {
	lastIndexedHeight := lastFullBlockHeight
	height := lastFullBlockHeight + 1
	for {
		submitted, err := s.IndexForHeight(ctx, height)
		if err != nil {
			return 0, err
		}
		if !submitted {
			return lastIndexedHeight, nil
		}

		lastIndexedHeight = height
		height++
	}
}
