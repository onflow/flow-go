package indexer

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/engine/access/collection_sync"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

// blockCollectionIndexerImpl implements BlockCollectionIndexer.
// It stores and indexes fetcher for a given block height.
type blockCollectionIndexerImpl struct {
	metrics     module.CollectionExecutedMetric
	lockManager lockctx.Manager
	db          storage.DB
	collections storage.Collections
}

var _ collection_sync.BlockCollectionIndexer = (*blockCollectionIndexerImpl)(nil)

// NewBlockCollectionIndexer creates a new BlockCollectionIndexer implementation.
//
// Parameters:
//   - metrics: Metrics collector for tracking collection indexing
//   - lockManager: Lock manager for coordinating database access
//   - db: Database for storage operations
//   - collections: collections storage for storing and indexing collections
//
// No error returns are expected during normal operation.
func NewBlockCollectionIndexer(
	metrics module.CollectionExecutedMetric,
	lockManager lockctx.Manager,
	db storage.DB,
	collections storage.Collections,
) *blockCollectionIndexerImpl {
	return &blockCollectionIndexerImpl{
		metrics:     metrics,
		lockManager: lockManager,
		db:          db,
		collections: collections,
	}
}

// IndexCollectionsForBlock stores and indexes collections for a given block height.
//
// No error returns are expected during normal operation.
func (bci *blockCollectionIndexerImpl) IndexCollectionsForBlock(
	_ uint64,
	cols []*flow.Collection,
) error {
	// Store and index collections
	return storage.WithLock(bci.lockManager, storage.LockInsertCollection, func(lctx lockctx.Context) error {
		return bci.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			for _, collection := range cols {
				// Store the collection, including constituent transactions, and index transactionID -> collectionID
				light, err := bci.collections.BatchStoreAndIndexByTransaction(lctx, collection, rw)
				if err != nil {
					return fmt.Errorf("failed to store collection: %w", err)
				}

				bci.metrics.CollectionFinalized(light)
				bci.metrics.CollectionExecuted(light)
			}
			return nil
		})
	})
}

// GetMissingCollections retrieves the block and returns collection guarantees that whose collections
// are missing in storage.
// Only garantees whose collections that are not already in storage are returned.
// No error returns are expected during normal operation.
func (bci *blockCollectionIndexerImpl) GetMissingCollections(block *flow.Block) ([]*flow.CollectionGuarantee, error) {
	var missingGuarantees []*flow.CollectionGuarantee
	for _, guarantee := range block.Payload.Guarantees {
		// Check if collection already exists in storage
		exists, err := bci.collections.ExistByID(guarantee.CollectionID)
		if err != nil {
			// Unexpected error
			return nil, fmt.Errorf("failed to check if collection %v exists: %w", guarantee.CollectionID, err)
		}

		if !exists {
			// Collection is missing
			missingGuarantees = append(missingGuarantees, guarantee)
		}
		// If collection exists, skip it
	}

	return missingGuarantees, nil
}
