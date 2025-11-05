package collections

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
	"github.com/rs/zerolog"
)

// ExecutionDataSyncer submits collections from execution data to the collections indexer. It is
// designed to be used within the collection syncer to optimize indexing when collection data is
// already available on the node.
type ExecutionDataSyncer struct {
	log                zerolog.Logger
	executionDataCache execution_data.ExecutionDataCache
	indexer            CollectionIndexer
}

func NewExecutionDataSyncer(
	log zerolog.Logger,
	executionDataCache execution_data.ExecutionDataCache,
	indexer CollectionIndexer,
) *ExecutionDataSyncer {
	return &ExecutionDataSyncer{
		log:                log.With().Str("component", "execution-data-syncer").Logger(),
		executionDataCache: executionDataCache,
		indexer:            indexer,
	}
}

// IndexForHeight indexes the collections for a given finalized block height using locally available
// execution data.
// Returns false and no error if execution data for the block is not available.
//
// Expected error returns during normal operation:
//   - [context.Canceled]: if the context is canceled before the collections are indexed.
func (s *ExecutionDataSyncer) IndexForHeight(ctx context.Context, height uint64) (bool, error) {
	executionData, err := s.executionDataCache.ByHeight(ctx, height)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) || execution_data.IsBlobNotFoundError(err) {
			return false, nil // data for the block is not available yet.
		}
		return false, fmt.Errorf("failed to get execution data for height %d: %w", height, err)
	}

	// index all standard (non-system) collections.
	standardCollections := executionData.StandardCollections()
	if len(standardCollections) > 0 {
		err = s.indexer.IndexCollections(standardCollections)
		if err != nil {
			return false, fmt.Errorf("failed to index collections from execution data for height %d: %w", height, err)
		}
	}

	s.log.Debug().
		Uint64("height", height).
		Int("collection_count", len(standardCollections)).
		Msg("indexed collections from execution data")

	return true, nil
}
