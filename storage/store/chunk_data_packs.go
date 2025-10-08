package store

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

type ChunkDataPacks struct {
	db             storage.DB
	collections    storage.Collections
	byChunkIDCache *Cache[flow.Identifier, *storage.StoredChunkDataPack]
}

var _ storage.ChunkDataPacks = (*ChunkDataPacks)(nil)

func NewChunkDataPacks(collector module.CacheMetrics, db storage.DB, collections storage.Collections, byChunkIDCacheSize uint) *ChunkDataPacks {

	storeWithLock := func(lctx lockctx.Proof, rw storage.ReaderBatchWriter, key flow.Identifier, val *storage.StoredChunkDataPack) error {
		return operation.InsertChunkDataPack(lctx, rw, val)
	}

	retrieve := func(r storage.Reader, key flow.Identifier) (*storage.StoredChunkDataPack, error) {
		var c storage.StoredChunkDataPack
		err := operation.RetrieveChunkDataPack(r, key, &c)
		return &c, err
	}

	cache := newCache(collector, metrics.ResourceChunkDataPack,
		withLimit[flow.Identifier, *storage.StoredChunkDataPack](byChunkIDCacheSize),
		withStoreWithLock(storeWithLock),
		withRetrieve(retrieve),
	)

	ch := ChunkDataPacks{
		db:             db,
		byChunkIDCache: cache,
		collections:    collections,
	}
	return &ch
}

// Remove removes multiple ChunkDataPacks cs keyed by their ChunkIDs in a batch.
// No errors are expected during normal operation, even if no entries are matched.
func (ch *ChunkDataPacks) Remove(chunkIDs []flow.Identifier) error {
	return ch.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		for _, c := range chunkIDs {
			err := ch.BatchRemove(c, rw)
			if err != nil {
				return fmt.Errorf("cannot remove chunk data pack: %w", err)
			}
		}

		return nil
	})
}

// StoreByChunkID stores multiple ChunkDataPacks cs keyed by their ChunkIDs in a batch.
// No errors are expected during normal operation, but it may return generic error
func (ch *ChunkDataPacks) StoreByChunkID(lctx lockctx.Proof, cs []*flow.ChunkDataPack) error {
	return ch.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		for _, c := range cs {
			sc := storage.ToStoredChunkDataPack(c)
			err := ch.byChunkIDCache.PutWithLockTx(lctx, rw, sc.ChunkID, sc)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// BatchRemove removes ChunkDataPack c keyed by its ChunkID in provided batch
// No errors are expected during normal operation, even if no entries are matched.
func (ch *ChunkDataPacks) BatchRemove(chunkID flow.Identifier, rw storage.ReaderBatchWriter) error {
	storage.OnCommitSucceed(rw, func() {
		ch.byChunkIDCache.Remove(chunkID)
	})
	return operation.RemoveChunkDataPack(rw.Writer(), chunkID)
}

func (ch *ChunkDataPacks) ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error) {
	schdp, err := ch.byChunkID(chunkID)
	if err != nil {
		return nil, err
	}

	chdp := &flow.ChunkDataPack{
		ChunkID:           schdp.ChunkID,
		StartState:        schdp.StartState,
		Proof:             schdp.Proof,
		ExecutionDataRoot: schdp.ExecutionDataRoot,
	}

	if !schdp.SystemChunk {
		collection, err := ch.collections.ByID(schdp.CollectionID)
		if err != nil {
			return nil, fmt.Errorf("could not retrive collection (id: %x) for stored chunk data pack: %w", schdp.CollectionID, err)
		}

		chdp.Collection = collection
	}

	return chdp, nil
}

func (ch *ChunkDataPacks) byChunkID(chunkID flow.Identifier) (*storage.StoredChunkDataPack, error) {
	schdp, err := ch.retrieveCHDP(chunkID)
	if err != nil {
		return nil, fmt.Errorf("could not retrive stored chunk data pack: %w", err)
	}

	return schdp, nil
}

func (ch *ChunkDataPacks) retrieveCHDP(chunkID flow.Identifier) (*storage.StoredChunkDataPack, error) {
	val, err := ch.byChunkIDCache.Get(ch.db.Reader(), chunkID)
	if err != nil {
		return nil, err
	}
	return val, nil
}
