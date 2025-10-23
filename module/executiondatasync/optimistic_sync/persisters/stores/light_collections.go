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
	data        []*flow.Collection
	collections storage.Collections
}

func NewCollectionsStore(
	data []*flow.Collection,
	collections storage.Collections,
) *LightCollectionsStore {
	return &LightCollectionsStore{
		data:        data,
		collections: collections,
	}
}

// Persist adds light collections to the batch.
// The caller must acquire [storage.LockInsertCollection] and hold it until the write batch has been
// committed.
//
// No error returns are expected during normal operations
func (c *LightCollectionsStore) Persist(lctx lockctx.Proof, batch storage.ReaderBatchWriter) error {
	for _, collection := range c.data {
		if _, err := c.collections.BatchStoreAndIndexByTransaction(lctx, collection, batch); err != nil {
			return fmt.Errorf("could not add light collections to batch: %w", err)
		}
	}

	return nil
}
