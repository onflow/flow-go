package store

import (
	"fmt"

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

	store := func(rw storage.ReaderBatchWriter, key flow.Identifier, val *storage.StoredChunkDataPack) error {
		return operation.InsertChunkDataPack(rw.Writer(), val)
	}

	retrieve := func(r storage.Reader, key flow.Identifier) (*storage.StoredChunkDataPack, error) {
		var c storage.StoredChunkDataPack
		err := operation.RetrieveChunkDataPack(r, key, &c)
		return &c, err
	}

	cache := newCache(collector, metrics.ResourceChunkDataPack,
		withLimit[flow.Identifier, *storage.StoredChunkDataPack](byChunkIDCacheSize),
		withStore(store),
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

// BatchStore stores ChunkDataPack c keyed by its ChunkID in provided batch.
// No errors are expected during normal operation, but it may return generic error
// if entity is not serializable or Badger unexpectedly fails to process request
func (ch *ChunkDataPacks) BatchStore(c *flow.ChunkDataPack, rw storage.ReaderBatchWriter) error {
	sc := storage.ToStoredChunkDataPack(c)
	storage.OnCommitSucceed(rw, func() {
		ch.byChunkIDCache.Insert(sc.ChunkID, sc)
	})
	return operation.InsertChunkDataPack(rw.Writer(), sc)
}

// Store stores multiple ChunkDataPacks cs keyed by their ChunkIDs in a batch.
// No errors are expected during normal operation, but it may return generic error
func (ch *ChunkDataPacks) Store(cs []*flow.ChunkDataPack) error {
	return ch.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		for _, c := range cs {
			err := ch.BatchStore(c, rw)
			if err != nil {
				return fmt.Errorf("cannot store chunk data pack: %w", err)
			}
		}

		return nil
	})
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
	reader, err := ch.db.Reader()
	if err != nil {
		return nil, err
	}
	val, err := ch.byChunkIDCache.Get(reader, chunkID)
	if err != nil {
		return nil, err
	}
	return val, nil
}
