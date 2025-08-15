package stores

import (
	"fmt"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
)

var _ PersisterStore = (*LightCollectionsStore)(nil)

// LightCollectionsStore handles persisting light collections
type LightCollectionsStore struct {
	inMemoryCollections  *unsynchronized.Collections
	persistedCollections storage.Collections
	lockManager          storage.LockManager
}

func NewCollectionsStore(
	inMemoryCollections *unsynchronized.Collections,
	persistedCollections storage.Collections,
	lockManager storage.LockManager,
) *LightCollectionsStore {
	return &LightCollectionsStore{
		inMemoryCollections:  inMemoryCollections,
		persistedCollections: persistedCollections,
		lockManager:          lockManager,
	}
}

// Persist adds light collections to the batch.
// No errors are expected during normal operations
func (c *LightCollectionsStore) Persist(batch storage.ReaderBatchWriter) error {
	lctx := c.lockManager.NewContext()
	err := lctx.AcquireLock(storage.LockInsertCollection)
	if err != nil {
		return fmt.Errorf("could not acquire lock for inserting light collections: %w", err)
	}
	defer lctx.Release()

	for _, collection := range c.inMemoryCollections.Data() {
		if _, err := c.persistedCollections.BatchStoreAndIndexByTransaction(lctx, &collection, batch); err != nil {
			return fmt.Errorf("could not add light collections to batch: %w", err)
		}
	}

	return nil
}
