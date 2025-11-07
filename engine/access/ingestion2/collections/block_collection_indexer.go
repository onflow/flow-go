package collections

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/engine/access/ingestion2"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

// blockCollectionIndexerImpl implements BlockCollectionIndexer.
// It stores and indexes collections for a given block height.
type blockCollectionIndexerImpl struct {
	metrics     module.CollectionExecutedMetric
	lockManager lockctx.Manager
	db          storage.DB
	collections storage.Collections
}

var _ ingestion2.BlockCollectionIndexer = (*blockCollectionIndexerImpl)(nil)

// NewBlockCollectionIndexer creates a new BlockCollectionIndexer implementation.
//
// Parameters:
//   - metrics: Metrics collector for tracking collection indexing
//   - lockManager: Lock manager for coordinating database access
//   - db: Database for storage operations
//   - collections: Collections storage for storing and indexing collections
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

// OnReceivedCollectionsForBlock stores and indexes collections for a given block height.
//
// No error returns are expected during normal operation.
func (bci *blockCollectionIndexerImpl) OnReceivedCollectionsForBlock(
	blockHeight uint64,
	cols []*flow.Collection,
) error {
	// Store and index collections
	err := storage.WithLock(bci.lockManager, storage.LockInsertCollection, func(lctx lockctx.Context) error {
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
	if err != nil {
		return fmt.Errorf("failed to index collections for block height %d: %w", blockHeight, err)
	}

	return nil
}
