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
	stored         storage.StoredChunkDataPacks
	byChunkIDCache *Cache[flow.Identifier, *storage.StoredChunkDataPack]
}

var _ storage.ChunkDataPacks = (*ChunkDataPacks)(nil)

func NewChunkDataPacks(collector module.CacheMetrics, db storage.DB, stored storage.StoredChunkDataPacks, collections storage.Collections, byChunkIDCacheSize uint) *ChunkDataPacks {

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
		stored:         stored,
		collections:    collections,
	}
	return &ch
}

// Remove removes multiple ChunkDataPacks cs keyed by their ChunkIDs in a batch.
// No errors are expected during normal operation, even if no entries are matched.
func (ch *ChunkDataPacks) Remove(chunkIDs []flow.Identifier) error {
	return ch.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		storage.OnCommitSucceed(rw, func() {
			// ch.stored.Remove(chunkIDs)
		})

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
func (ch *ChunkDataPacks) Store(cs []*flow.ChunkDataPack) (
	func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error, error) {
	// store chunk data packs in separate storage

	storedChunkDataPacks := storage.ToStoredChunkDataPacks(cs)

	storedChunkDataPackIDs, err := ch.stored.StoreChunkDataPacks(storedChunkDataPacks)
	if err != nil {
		return nil, fmt.Errorf("cannot store chunk data packs: %w", err)
	}

	if len(cs) != len(storedChunkDataPackIDs) {
		return nil, fmt.Errorf("stored chunk data pack IDs count mismatch: expected: %d, got: %d: %w",
			len(cs), len(storedChunkDataPackIDs), storage.ErrDataMismatch)
	}

	return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
		for i, c := range cs {
			storedChunkDataPackID := storedChunkDataPackIDs[i]
			// index stored chunk data pack ID by chunk ID
			err := operation.InsertChunkDataPackID(lctx, rw, c.ChunkID, storedChunkDataPackID)
			if err != nil {
				return fmt.Errorf("cannot index stored chunk data pack ID by chunk ID: %w", err)
			}
		}

		return nil
	}, nil
}

// BatchRemove removes ChunkDataPack c keyed by its ChunkID in provided batch
// No errors are expected during normal operation, even if no entries are matched.
// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (ch *ChunkDataPacks) BatchRemove(chunkID flow.Identifier, rw storage.ReaderBatchWriter) error {
	storage.OnCommitSucceed(rw, func() {
		ch.byChunkIDCache.Remove(chunkID)
	})
	return operation.RemoveChunkDataPack(rw.Writer(), chunkID)
}

func (ch *ChunkDataPacks) ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error) {
	StoredChunkDataPackID, err := operation.RetrieveChunkDataPackID(ch.db.Reader(), chunkID)
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
