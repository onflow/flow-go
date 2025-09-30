package store

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

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

	cache := newCache(collector, metrics.ResourceStoredChunkDataPack,
		withLimit[flow.Identifier, *storage.StoredChunkDataPack](byIDCacheSize),
		withStore(operation.InsertStoredChunkDataPack),
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
func (ch *StoredChunkDataPacks) ByID(storedChunkDataPackID flow.Identifier) (*storage.StoredChunkDataPack, error) {
	val, err := ch.byIDCache.Get(ch.db.Reader(), storedChunkDataPackID)
	if err != nil {
		return nil, err
	}
	return val, nil
}

// Remove removes multiple StoredChunkDataPacks cs keyed by their IDs in a batch.
// No errors are expected during normal operation, even if no entries are matched.
func (ch *StoredChunkDataPacks) Remove(ids []flow.Identifier) error {
	return ch.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		for _, id := range ids {
			err := ch.batchRemove(id, rw)
			if err != nil {
				return fmt.Errorf("cannot remove chunk data pack: %w", err)
			}
		}

		return nil
	})
}

func (ch *StoredChunkDataPacks) batchRemove(storedChunkDataPackID flow.Identifier, rw storage.ReaderBatchWriter) error {
	storage.OnCommitSucceed(rw, func() {
		ch.byIDCache.Remove(storedChunkDataPackID)
	})
	return operation.RemoveStoredChunkDataPack(rw.Writer(), storedChunkDataPackID)
}

// StoreChunkDataPacks stores multiple StoredChunkDataPacks cs in a batch.
// It returns the IDs of the stored chunk data packs.
// No error are expected during normal operation.
func (ch *StoredChunkDataPacks) StoreChunkDataPacks(cs []*storage.StoredChunkDataPack) ([]flow.Identifier, error) {
	ids := make([]flow.Identifier, 0, len(cs))

	if len(cs) == 0 {
		return ids, nil
	}

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
