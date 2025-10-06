package stores

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var _ PersisterStore = (*LightCollectionsStore)(nil)

// LightCollectionsStore handles persisting light collections
type LightCollectionsStore struct {
	data                 []*flow.Collection
	persistedCollections storage.Collections
}

func NewCollectionsStore(
	data []*flow.Collection,
	persistedCollections storage.Collections,
	lockManager storage.LockManager,
) *LightCollectionsStore {
	return &LightCollectionsStore{
		data:                 data,
		persistedCollections: persistedCollections,
	}
}

// Persist adds light collections to the batch.
//
// No error returns are expected during normal operations
func (c *LightCollectionsStore) Persist(lctx lockctx.Proof, batch storage.ReaderBatchWriter) error {
	for _, collection := range c.data {
		if _, err := c.persistedCollections.BatchStoreAndIndexByTransaction(lctx, collection, batch); err != nil {
			return fmt.Errorf("could not add light collections to batch: %w", err)
		}
	}

	return nil
}
