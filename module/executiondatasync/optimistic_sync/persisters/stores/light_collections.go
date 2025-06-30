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
}

func NewCollectionsStore(
	inMemoryCollections *unsynchronized.Collections,
	persistedCollections storage.Collections,
) *LightCollectionsStore {
	return &LightCollectionsStore{
		inMemoryCollections:  inMemoryCollections,
		persistedCollections: persistedCollections,
	}
}

// Persist adds light collections to the batch.
// No errors are expected during normal operations
func (c *LightCollectionsStore) Persist(batch storage.ReaderBatchWriter) error {
	for _, collection := range c.inMemoryCollections.LightCollections() {
		if err := c.persistedCollections.BatchStoreLightAndIndexByTransaction(&collection, batch); err != nil {
			return fmt.Errorf("could not add light collections to batch: %w", err)
		}
	}

	return nil
}
