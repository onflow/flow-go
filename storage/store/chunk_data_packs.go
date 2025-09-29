package store

import (
	"errors"
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
	byChunkIDCache *Cache[flow.Identifier, flow.Identifier] // cache chunkID -> storedChunkDataPackID
}

var _ storage.ChunkDataPacks = (*ChunkDataPacks)(nil)

func NewChunkDataPacks(collector module.CacheMetrics, db storage.DB, stored storage.StoredChunkDataPacks, collections storage.Collections, byChunkIDCacheSize uint) *ChunkDataPacks {

	storeWithLock := func(lctx lockctx.Proof, rw storage.ReaderBatchWriter, key flow.Identifier, val flow.Identifier) error {
		return operation.InsertChunkDataPackID(lctx, rw, key, val)
	}

	retrieve := func(r storage.Reader, key flow.Identifier) (flow.Identifier, error) {
		var storedChunkDataPackID flow.Identifier
		err := operation.RetrieveChunkDataPackID(r, key, &storedChunkDataPackID)
		return storedChunkDataPackID, err
	}

	cache := newCache(collector, metrics.ResourceChunkDataPack,
		withLimit[flow.Identifier, flow.Identifier](byChunkIDCacheSize),
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

// NewChunkDataPacksSimple creates a ChunkDataPacks instance with a simple constructor for backward compatibility.
// This constructor creates its own StoredChunkDataPacks instance internally.
func NewChunkDataPacksSimple(collector module.CacheMetrics, db storage.DB, collections storage.Collections, byChunkIDCacheSize uint) *ChunkDataPacks {
	stored := NewStoredChunkDataPacks(collector, db, byChunkIDCacheSize)
	return NewChunkDataPacks(collector, db, stored, collections, byChunkIDCacheSize)
}

// Remove removes multiple ChunkDataPacks cs keyed by their ChunkIDs in a batch.
// No errors are expected during normal operation, even if no entries are matched.
func (ch *ChunkDataPacks) Remove(chunkIDs []flow.Identifier) error {
	return ch.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		storage.OnCommitSucceed(rw, func() {
			// ch.stored.Remove(chunkIDs)
		})

		return ch.BatchRemove(chunkIDs, rw)
	})
}

// Store
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

	storeChunkDataPacksFunc := func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
		for i, c := range cs {
			storedChunkDataPackID := storedChunkDataPackIDs[i]
			// index stored chunk data pack ID by chunk ID
			err := operation.InsertChunkDataPackID(lctx, rw, c.ChunkID, storedChunkDataPackID)
			if err != nil {
				return fmt.Errorf("cannot index stored chunk data pack ID by chunk ID: %w", err)
			}
		}

		return nil
	}
	return storeChunkDataPacksFunc, nil
}

// BatchRemove removes ChunkDataPack c keyed by its ChunkID in provided batch
// No errors are expected during normal operation, even if no entries are matched.
func (ch *ChunkDataPacks) BatchRemove(chunkIDs []flow.Identifier, batch storage.ReaderBatchWriter) error {
	// First, collect all stored chunk data pack IDs that need to be removed
	var storedChunkDataPackIDs []flow.Identifier
	for _, chunkID := range chunkIDs {
		storedChunkDataPackID, err := ch.byChunkIDCache.Get(batch.GlobalReader(), chunkID) // remove from cache optimistically
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				// If we can't find the stored chunk data pack ID, continue with other removals
				// This handles the case where the chunk data pack was never properly stored
				continue
			}

			return fmt.Errorf("cannot retrieve stored chunk data pack ID for chunk %x: %w", chunkID, err)
		}
		storedChunkDataPackIDs = append(storedChunkDataPackIDs, storedChunkDataPackID)
	}

	// Remove the stored chunk data packs
	if len(storedChunkDataPackIDs) > 0 {
		err := ch.stored.Remove(storedChunkDataPackIDs)
		if err != nil {
			return fmt.Errorf("cannot remove stored chunk data packs: %w", err)
		}
	}

	// Remove the chunk data pack ID mappings and update cache
	for _, chunkID := range chunkIDs {
		storage.OnCommitSucceed(batch, func() {
			ch.byChunkIDCache.Remove(chunkID)
		})
		err := operation.RemoveChunkDataPackID(batch.Writer(), chunkID)
		if err != nil {
			return fmt.Errorf("cannot remove chunk data pack %x: %w", chunkID, err)
		}
	}
	return nil
}

func (ch *ChunkDataPacks) ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error) {
	// First, retrieve the stored chunk data pack ID (using cache if available)
	storedChunkDataPackID, err := ch.byChunkIDCache.Get(ch.db.Reader(), chunkID)
	if err != nil {
		return nil, err
	}

	// Then retrieve the actual stored chunk data pack using the ID
	schdp, err := ch.stored.ByID(storedChunkDataPackID)
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
			return nil, fmt.Errorf("could not retrieve collection (id: %x) for stored chunk data pack: %w", schdp.CollectionID, err)
		}

		chdp.Collection = collection
	}

	return chdp, nil
}

// StoreByChunkID stores multiple ChunkDataPacks cs keyed by their ChunkIDs in a batch.
// This is a convenience method that wraps the Store method for backward compatibility.
func (ch *ChunkDataPacks) StoreByChunkID(lctx lockctx.Proof, cs []*flow.ChunkDataPack) error {
	storeFunc, err := ch.Store(cs)
	if err != nil {
		return err
	}
	return ch.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return storeFunc(lctx, rw)
	})
}
