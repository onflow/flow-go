package store

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// StoredChunkDataPacks represents persistent storage for chunk data packs.
// It works with the reduced representation `StoredChunkDataPack` for chunk data packs,
// where instead of the full collection data, only the collection's hash (ID) is contained.
type StoredChunkDataPacks struct {
	db        storage.DB
	byIDCache *Cache[flow.Identifier, *storage.StoredChunkDataPack]
}

var _ storage.StoredChunkDataPacks = (*StoredChunkDataPacks)(nil)

func NewStoredChunkDataPacks(collector module.CacheMetrics, db storage.DB, byIDCacheSize uint) *StoredChunkDataPacks {

	retrieve := func(r storage.Reader, key flow.Identifier) (*storage.StoredChunkDataPack, error) {
		var c storage.StoredChunkDataPack
		err := operation.RetrieveStoredChunkDataPack(r, key, &c)
		return &c, err
	}

	cache := newCache(collector, metrics.ResourceChunkDataPack,
		withLimit[flow.Identifier, *storage.StoredChunkDataPack](byIDCacheSize),
		withStore(operation.InsertStoredChunkDataPack),
		withRemove[flow.Identifier, *storage.StoredChunkDataPack](func(rw storage.ReaderBatchWriter, id flow.Identifier) error {
			return operation.RemoveChunkDataPack(rw.Writer(), id)
		}),
		withRetrieve(retrieve),
	)

	ch := StoredChunkDataPacks{
		db:        db,
		byIDCache: cache,
	}
	return &ch
}

// ByID returns the StoredChunkDataPack for the given ID.
// It returns [storage.ErrNotFound] if no entry exists for the given ID.
func (ch *StoredChunkDataPacks) ByID(chunkDataPackID flow.Identifier) (*storage.StoredChunkDataPack, error) {
	val, err := ch.byIDCache.Get(ch.db.Reader(), chunkDataPackID)
	if err != nil {
		return nil, err
	}
	return val, nil
}

// Remove removes multiple StoredChunkDataPacks cs keyed by their IDs in a batch.
// No error returns are expected during normal operation, even if none of the referenced objects exist in storage.
func (ch *StoredChunkDataPacks) Remove(ids []flow.Identifier) error {
	return ch.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		for _, id := range ids {
			err := ch.batchRemove(id, rw)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// BatchRemove removes multiple ChunkDataPacks with the given IDs from storage as part of the provided write batch.
// No error returns are expected during normal operation, even if no entries are matched.
func (ch *StoredChunkDataPacks) BatchRemove(chunkDataPackIDs []flow.Identifier, rw storage.ReaderBatchWriter) error {
	for _, id := range chunkDataPackIDs {
		err := ch.batchRemove(id, rw)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ch *StoredChunkDataPacks) batchRemove(chunkDataPackID flow.Identifier, rw storage.ReaderBatchWriter) error {
	return ch.byIDCache.RemoveTx(rw, chunkDataPackID)
}

// StoreChunkDataPacks stores multiple StoredChunkDataPacks cs in a batch.
// It returns the chunk data pack IDs
// No error returns are expected during normal operation.
func (ch *StoredChunkDataPacks) StoreChunkDataPacks(cs []*storage.StoredChunkDataPack) ([]flow.Identifier, error) {
	if len(cs) == 0 {
		return nil, nil
	}
	ids := make([]flow.Identifier, 0, len(cs))

	err := ch.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		for _, sc := range cs {
			id := sc.ID()
			err := ch.byIDCache.PutTx(rw, id, sc)
			if err != nil {
				return err
			}
			ids = append(ids, id)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("cannot store chunk data packs: %w", err)
	}
	return ids, nil
}
