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
	// the protocol DB is used for storing index mappings from chunk ID to stored chunk data pack ID
	db storage.DB

	// the actual chunk data pack is stored here, which is a separate storage from protocol DB
	stored storage.StoredChunkDataPacks

	// the collection is stored separately by the caller, it is only used here for retrieving collection
	// when returning chunk data pack by chunk ID
	collections storage.Collections

	// cache chunkID -> storedChunkDataPackID
	chunkIDToStoredChunkDataPackIDCache *Cache[flow.Identifier, flow.Identifier]

	// it takes 3 look ups to return chunk data pack by chunk ID:
	// 1. a cache lookup for chunkID -> storedChunkDataPackID
	// 2. a lookup for storedChunkDataPackID -> StoredChunkDataPack (only has CollectionID, no collection data)
	// 3. a lookup for CollectionID -> Collection, then restore the chunk data pack with the collection and the StoredChunkDataPack
}

var _ storage.ChunkDataPacks = (*ChunkDataPacks)(nil)

func NewChunkDataPacks(collector module.CacheMetrics, db storage.DB, stored storage.StoredChunkDataPacks, collections storage.Collections, chunkIDToStoredChunkDataPackIDCacheSize uint) *ChunkDataPacks {

	storeWithLock := func(lctx lockctx.Proof, rw storage.ReaderBatchWriter, key flow.Identifier, val flow.Identifier) error {
		return operation.InsertChunkDataPackID(lctx, rw, key, val)
	}

	retrieve := func(r storage.Reader, key flow.Identifier) (flow.Identifier, error) {
		var storedChunkDataPackID flow.Identifier
		err := operation.RetrieveChunkDataPackID(r, key, &storedChunkDataPackID)
		return storedChunkDataPackID, err
	}

	cache := newCache(collector, metrics.ResourceChunkDataPack,
		withLimit[flow.Identifier, flow.Identifier](chunkIDToStoredChunkDataPackIDCacheSize),
		withStoreWithLock(storeWithLock),
		withRetrieve(retrieve),
	)

	ch := ChunkDataPacks{
		db:                                  db,
		chunkIDToStoredChunkDataPackIDCache: cache,
		stored:                              stored,
		collections:                         collections,
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

		return ch.BatchRemove(chunkIDs, rw)
	})
}

// Store stores multiple ChunkDataPacks in a two-phase process:
// 1. First phase: Store chunk data packs (StoredChunkDataPack) by its hash (storedChunkDataPackID) in chunk data pack database.
// 2. Second phase: Create index mappings from ChunkID to storedChunkDataPackID in protocol database
//
// The reason it's a two-phase process is that, the chunk data pack and the other execution data are stored in different databases.
// The two-phase approach ensures that:
//   - Chunk data pack content is stored atomically in the chunk data pack database
//   - Index mappings are created within the same atomic batch update in protocol database
//
// The Store method returns:
//   - func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error: Function to index the chunk id with
//     chunk data pack hash within batch update to store along with other execution data into protocol database,
//     this function might return [storage.ErrDataMismatch] when an existing chunk data pack ID is found for
//     the same chunk ID, and is different from the one being stored.
//     the caller must acquire [storage.LockInsertChunkDataPack] and hold it until the database write has been committed.
//   - error: No error should be returned during normal operation. Any error indicates a failure in the first phase.
func (ch *ChunkDataPacks) Store(cs []*flow.ChunkDataPack) (
	func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error, error) {

	// Phase 1: Store chunk data packs in the separate stored storage layer
	// This converts the ChunkDataPacks to StoredChunkDataPacks format and stores them
	storedChunkDataPacks := storage.ToStoredChunkDataPacks(cs)

	// Store the chunk data packs and get back their unique IDs
	storedChunkDataPackIDs, err := ch.stored.StoreChunkDataPacks(storedChunkDataPacks)
	if err != nil {
		return nil, fmt.Errorf("cannot store chunk data packs: %w", err)
	}

	// Sanity check: validate that we received the expected number of IDs
	if len(cs) != len(storedChunkDataPackIDs) {
		return nil, fmt.Errorf("stored chunk data pack IDs count mismatch: expected: %d, got: %d",
			len(cs), len(storedChunkDataPackIDs))
	}

	// Phase 2: Create the function that will index chunkID -> storedChunkDataPackID mappings
	storeChunkDataPacksFunc := func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
		// Create index mappings for each chunk data pack
		for i, c := range cs {
			storedChunkDataPackID := storedChunkDataPackIDs[i]
			// Index the stored chunk data pack ID by chunk ID for fast retrieval
			err := operation.InsertChunkDataPackID(lctx, rw, c.ChunkID, storedChunkDataPackID)
			if err != nil {
				return fmt.Errorf("cannot index stored chunk data pack ID by chunk ID: %w", err)
			}
		}

		return nil
	}

	// Return the function that completes the storage process
	return storeChunkDataPacksFunc, nil
}

// BatchRemove removes ChunkDataPack c keyed by its ChunkID in provided batch
// No errors are expected during normal operation, even if no entries are matched.
func (ch *ChunkDataPacks) BatchRemove(chunkIDs []flow.Identifier, batch storage.ReaderBatchWriter) error {
	// First, collect all stored chunk data pack IDs that need to be removed
	var storedChunkDataPackIDs []flow.Identifier
	for _, chunkID := range chunkIDs {
		storedChunkDataPackID, err := ch.chunkIDToStoredChunkDataPackIDCache.Get(batch.GlobalReader(), chunkID) // remove from cache optimistically
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
			ch.chunkIDToStoredChunkDataPackIDCache.Remove(chunkID)
		})
		err := operation.RemoveChunkDataPackID(batch.Writer(), chunkID)
		if err != nil {
			return fmt.Errorf("cannot remove chunk data pack %x: %w", chunkID, err)
		}
	}
	return nil
}

// ByChunkID returns the chunk data for the given chunk ID.
// It returns [storage.ErrNotFound] if no entry exists for the given chunk ID.
func (ch *ChunkDataPacks) ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error) {
	// First, retrieve the stored chunk data pack ID (using cache if available)
	storedChunkDataPackID, err := ch.chunkIDToStoredChunkDataPackIDCache.Get(ch.db.Reader(), chunkID)
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve stored chunk data pack ID for chunk %x: %w", chunkID, err)
	}

	// Then retrieve the actual stored chunk data pack using the ID
	schdp, err := ch.stored.ByID(storedChunkDataPackID)
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve stored chunk data pack %x for chunk %x: %w", storedChunkDataPackID, chunkID, err)
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
