package store

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

type ChunkDataPacks struct {
	// the protocol DB is used for storing index mappings from chunk ID to chunk data pack ID
	protocolDB storage.DB

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

	retrieve := func(r storage.Reader, chunkID flow.Identifier) (flow.Identifier, error) {
		var storedChunkDataPackID flow.Identifier
		err := operation.RetrieveChunkDataPackID(r, chunkID, &storedChunkDataPackID)
		return storedChunkDataPackID, err
	}

	cache := newCache(collector, metrics.ResourceChunkIDToChunkDataPackIndex,
		withLimit[flow.Identifier, flow.Identifier](chunkIDToStoredChunkDataPackIDCacheSize),
		withStoreWithLock(operation.IndexChunkDataPackByChunkID),
		withRetrieve(retrieve),
	)

	ch := ChunkDataPacks{
		protocolDB:                          db,
		chunkIDToStoredChunkDataPackIDCache: cache,
		stored:                              stored,
		collections:                         collections,
	}
	return &ch
}

// Store persists multiple ChunkDataPacks in a two-phase process:
// 1. Store chunk data packs (StoredChunkDataPack) by its hash (storedChunkDataPackID) in chunk data pack database.
// 2. Populate index mapping from ChunkID to storedChunkDataPackID in protocol database.
//
// Reasoning for two-phase approach: the chunk data pack and the other execution data are stored in different databases.
//   - Chunk data pack content is stored in the chunk data pack database by its hash (ID). Conceptually, it would be possible
//     to store multiple different (disagreeing) chunk data packs here. Each chunk data pack is stored using its own collision
//     resistant hash as key, so different chunk data packs will be stored under different keys. So from the perspective of the
//     storage layer, we _could_ in phase 1 store all known chunk data packs. However, an Execution Node may only commit to a single
//     chunk data pack (or it will get slashed). This mapping from chunk ID to the ID of the chunk data pack that the Execution Node
//     actually committed to is stored in the protocol database, in the following phase 2.
//   - In the second phase, we populate the index mappings from ChunkID to one "distinguished" chunk data pack ID. This mapping
//     is stored in the protocol database. Typically, en Execution Node uses this for indexing its own chunk data packs which it
//     publicly committed to.
//   - This function can approximately be described as an atomic operation. When it completes successfully, either both databases
//     have been updated, or neither. However, this is an approximation only, because interim states exist, where the chunk data
//     packs already have been stored in the chunk data pack database, but the index mappings do not yet exist.
//
// The Store method returns:
//   - func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error: Function for populating the index mapping from chunkID
//     to chunk data pack ID in the protocol database. This mapping persists that the Execution Node committed to the result
//     represented by this chunk data pack. This function returns [storage.ErrDataMismatch] when a _different_ chunk data pack
//     ID for the same chunk ID has already been stored (changing which result an execution Node committed to would be a
//     slashable protocol violation). The caller must acquire [storage.LockInsertChunkDataPack] and hold it until the database
//     write has been committed.
//   - error: No error should be returned during normal operation. Any error indicates a failure in the first phase.
func (ch *ChunkDataPacks) Store(cs []*flow.ChunkDataPack) (
	func(lctx lockctx.Proof, protocolDBBatch storage.ReaderBatchWriter) error,
	error,
) {

	// Phase 1: Store chunk data packs in dedicated (separate) database. This converts the
	// ChunkDataPacks to the reduced StoredChunkDataPacks representation and stores them.
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
	storeChunkDataPacksFunc := func(lctx lockctx.Proof, protocolDBBatch storage.ReaderBatchWriter) error {
		protocolDBBatch.AddCallback(func(err error) {
			if err != nil {
				log.Warn().Err(err).Msgf("batch operation failed, rolling back stored chunk data packs for chunks: %v", cs)
				// Rollback the stored chunk data packs if the batch operation fails
				err := ch.stored.Remove(storedChunkDataPackIDs) // rollback stored chunk data packs on failure
				if err != nil {
					log.Fatal().Err(err).Msgf("cannot rollback stored chunk data packs") // log the error, but do not override the original error
				}
			}
		})

		// Create index mappings for each chunk data pack
		for i, c := range cs {
			storedChunkDataPackID := storedChunkDataPackIDs[i]
			// Index the stored chunk data pack ID by chunk ID for fast retrieval
			err := ch.chunkIDToStoredChunkDataPackIDCache.PutWithLockTx(
				lctx, protocolDBBatch, c.ChunkID, storedChunkDataPackID)
			if err != nil {
				return fmt.Errorf("cannot index stored chunk data pack ID by chunk ID: %w", err)
			}
		}

		return nil
	}

	// Return the function that completes the storage process
	return storeChunkDataPacksFunc, nil
}

// BatchRemove remove multiple ChunkDataPacks with the given chunk IDs.
// It performs a two-phase removal:
// 1. First phase: Remove index mappings from ChunkID to storedChunkDataPackID in the protocol database
// 2. Second phase: Remove chunk data packs (StoredChunkDataPack) by its hash (storedChunkDataPackID) in chunk data pack database.
// Note: it does not remove the collection referred by the chunk data pack.
// This method is useful for the rollback execution tool to batch remove chunk data packs associated with a set of blocks.
// No errors are expected during normal operation, even if no entries are matched.
func (ch *ChunkDataPacks) BatchRemove(
	chunkIDs []flow.Identifier,
	protocolDBBatch storage.ReaderBatchWriter,
	chunkDataPackDBBatch storage.ReaderBatchWriter,
) error {
	// First, collect all stored chunk data pack IDs that need to be removed
	var storedChunkDataPackIDs []flow.Identifier
	for _, chunkID := range chunkIDs {
		storedChunkDataPackID, err := ch.chunkIDToStoredChunkDataPackIDCache.Get(protocolDBBatch.GlobalReader(), chunkID)
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
		storage.OnCommitSucceed(protocolDBBatch, func() {
			// no errors expected during normal operation, even if no entries exist with the given IDs
			err := ch.stored.BatchRemove(storedChunkDataPackIDs, chunkDataPackDBBatch)
			if err != nil {
				panic("cannot remove stored chunk data packs: " + err.Error())
			}
		})
	}

	// Remove the chunk data pack ID mappings and update cache
	for _, chunkID := range chunkIDs {
		ch.chunkIDToStoredChunkDataPackIDCache.RemoveTx(protocolDBBatch, chunkID)
	}
	return nil
}

// BatchRemoveChunkDataPacksOnly removes multiple ChunkDataPacks with the given chunk IDs from chunk data pack database only.
// It does not remove the index mappings from ChunkID to storedChunkDataPackID in the protocol database.
// This method is useful for the runtime chunk data pack pruner to batch remove chunk data packs associated with a set of blocks.
// No errors are expected during normal operation, even if no entries are matched.
func (ch *ChunkDataPacks) BatchRemoveChunkDataPacksOnly(chunkIDs []flow.Identifier, chunkDataPackDBBatch storage.ReaderBatchWriter) error {
	// First, collect all stored chunk data pack IDs that need to be removed
	var storedChunkDataPackIDs []flow.Identifier
	for _, chunkID := range chunkIDs {
		storedChunkDataPackID, err := ch.chunkIDToStoredChunkDataPackIDCache.Get(chunkDataPackDBBatch.GlobalReader(), chunkID) // remove from cache optimistically
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
		err := ch.stored.BatchRemove(storedChunkDataPackIDs, chunkDataPackDBBatch)
		if err != nil {
			return fmt.Errorf("cannot remove stored chunk data packs: %w", err)
		}
	}

	// Remove from cache
	for _, chunkID := range chunkIDs {
		chunkDataPackDBBatch.AddCallback(func(err error) {
			if err == nil {
				ch.chunkIDToStoredChunkDataPackIDCache.Remove(chunkID)
			}
		})
	}

	return nil
}

// ByChunkID returns the chunk data for the given chunk ID.
// It returns [storage.ErrNotFound] if no entry exists for the given chunk ID.
func (ch *ChunkDataPacks) ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error) {
	// First, retrieve the stored chunk data pack ID (using cache if available)
	storedChunkDataPackID, err := ch.chunkIDToStoredChunkDataPackIDCache.Get(ch.protocolDB.Reader(), chunkID)
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve stored chunk data pack ID for chunk %x: %w", chunkID, err)
	}

	// Then retrieve the reduced representation of the chunk data pack via its ID.
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
