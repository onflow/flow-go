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

// ChunkDataPacks manages storage and retrieval of ChunkDataPacks, primarily serving the use case of EXECUTION NODES persisting
// and indexing chunk data packs for their OWN RESULTS. Essentially, the chunk describes a batch of work to be done, and the
// chunk data pack describes the result of that work. The storage of chunk data packs is segregated across different
// storage components for efficiency and modularity reasons:
//  0. Usually (ignoring the system chunk for a moment), the batch of work is given by the collection referenced in the chunk
//     data pack. For any chunk data pack being stored, we assume that the executed collection has *previously* been persisted
//     in [storage.Collections]. It is useful to persist the collections individually, so we can individually retrieve them.
//  1. The actual chunk data pack itself is stored in a dedicated storage component `cdpStorage`. Note that for this storage
//     component, no atomicity is required, as we are storing chunk data packs by their collision-resistant hashes, so
//     different chunk data packs will be stored under different keys.
//     Theoretically, nodes could store persist multiple different (disagreeing) chunk data packs for the same
//     chunk in this step. However, for efficiency, Execution Nodes only store their own chunk data packs.
//  2. The index mapping from ChunkID to chunkDataPackID is stored in the protocol database for fast retrieval.
//     This index is intended to be populated by execution nodes when they commit to a specific result represented by the chunk
//     data pack. Here, we require atomicity, as an execution node should not be changing / overwriting which chunk data pack
//     it committed to (during normal operations).
//
// Since the executed collections are stored separately (step 0, above), we can just use the collection ID in context of the
// chunk data pack storage (step 1, above). Therefore, we utilize the reduced representation [storage.StoredChunkDataPack]
// internally. While removing redundant data from storage, it takes 3 look-ups to return chunk data pack by chunk ID:
//
//	i. a lookup for chunkID -> chunkDataPackID
//	ii. a lookup for chunkDataPackID -> StoredChunkDataPack (only has CollectionID, no collection data)
//	iii. a lookup for CollectionID -> Collection, then reconstruct the chunk data pack from the collection and the StoredChunkDataPack
type ChunkDataPacks struct {
	// the protocol DB is used for storing index mappings from chunk ID to chunk data pack ID
	protocolDB storage.DB

	// the actual chunk data pack is stored here, which is a separate storage from protocol DB
	stored storage.StoredChunkDataPacks

	// We assume that for every chunk data pack not for a system chunk, the executed collection has 
	// previously been persisted in `storage.Collections`. We use this storage abstraction here only for
	// retrieving collections. We assume that `storage.Collections` has its own caching already built in.  
	collections storage.Collections

	// cache chunkID -> chunkDataPackID
	chunkIDToChunkDataPackIDCache *Cache[flow.Identifier, flow.Identifier]
}

var _ storage.ChunkDataPacks = (*ChunkDataPacks)(nil)

func NewChunkDataPacks(collector module.CacheMetrics, db storage.DB, stored storage.StoredChunkDataPacks, collections storage.Collections, chunkIDToChunkDataPackIDCacheSize uint) *ChunkDataPacks {

	retrieve := func(r storage.Reader, chunkID flow.Identifier) (flow.Identifier, error) {
		var chunkDataPackID flow.Identifier
		err := operation.RetrieveChunkDataPackID(r, chunkID, &chunkDataPackID)
		return chunkDataPackID, err
	}

	remove := func(rw storage.ReaderBatchWriter, chunkID flow.Identifier) error {
		return operation.RemoveChunkDataPackID(rw.Writer(), chunkID)
	}

	cache := newCache(collector, metrics.ResourceChunkIDToChunkDataPackIndex,
		withLimit[flow.Identifier, flow.Identifier](chunkIDToChunkDataPackIDCacheSize),
		withStoreWithLock(operation.IndexChunkDataPackByChunkID),
		withRemove[flow.Identifier, flow.Identifier](remove),
		withRetrieve(retrieve),
	)

	ch := ChunkDataPacks{
		protocolDB:                    db,
		chunkIDToChunkDataPackIDCache: cache,
		stored:                        stored,
		collections:                   collections,
	}
	return &ch
}

// Store persists multiple ChunkDataPacks in a two-phase process:
// 1. Store chunk data packs (StoredChunkDataPack) by its hash (chunkDataPackID) in chunk data pack database.
// 2. Populate index mapping from ChunkID to chunkDataPackID in protocol database.
//
// Reasoning for two-phase approach: the chunk data pack and the other execution data are stored in different databases.
//   - Chunk data pack content is stored in the chunk data pack database by its hash (ID). Conceptually, it would be possible
//     to store multiple different (disagreeing) chunk data packs here. Each chunk data pack is stored using its own collision
//     resistant hash as key, so different chunk data packs will be stored under different keys. So from the perspective of the
//     storage layer, we _could_ in phase 1 store all known chunk data packs. However, an Execution Node may only commit to a single
//     chunk data pack (or it will get slashed). This mapping from chunk ID to the ID of the chunk data pack that the Execution Node
//     actually committed to is stored in the protocol database, in the following phase 2.
//   - In the second phase, we populate the index mappings from ChunkID to one "distinguished" chunk data pack ID. This mapping
//     is stored in the protocol database. Typically, an Execution Node uses this for indexing its own chunk data packs which it
//     publicly committed to.
//
// ATOMICITY:
// [ChunkDataPacks.Store] executes phase 1 immediately, persisting the chunk data packs in their dedicated database. However,
// the index mappings in phase 2 is deferred to the caller, who must invoke the returned functor to perform phase 2. This
// approach has the following benefits:
//   - Our API reflects that we are writing to two different databases here, with the chunk data pack database containing largely
//     specialized data subject to pruning. In contrast, the protocol database persists the commitments a node make (subject to
//     slashing). The caller receives the ability to persist this commitment in the form of the returned functor. The functor
//     may be discarded by the caller without corrupting the state (if anything, we have just stored some additional chunk data
//     packs).
//   - The serialization and storage of the comparatively large chunk data packs is separated from the protocol database writes.
//   - The locking duration of the protocol database is reduced.
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
	chunkDataPackIDs, err := ch.stored.StoreChunkDataPacks(storedChunkDataPacks)
	if err != nil {
		return nil, fmt.Errorf("cannot store chunk data packs: %w", err)
	}

	// Sanity check: validate that we received the expected number of IDs
	if len(cs) != len(chunkDataPackIDs) {
		return nil, fmt.Errorf("stored chunk data pack IDs count mismatch: expected: %d, got: %d",
			len(cs), len(chunkDataPackIDs))
	}

	// Phase 2: Create the function that will index chunkID -> chunkDataPackID mappings
	storeChunkDataPacksFunc := func(lctx lockctx.Proof, protocolDBBatch storage.ReaderBatchWriter) error {
		protocolDBBatch.AddCallback(func(err error) {
			if err != nil {
				log.Warn().Err(err).Msgf("indexing chunkID -> chunkDataPackID mapping failed, chunkDataPackIDs: %v", chunkDataPackIDs)
			}
		})

		// Create index mappings for each chunk data pack
		for i, c := range cs {
			chunkDataPackID := chunkDataPackIDs[i]
			// Index the stored chunk data pack ID by chunk ID for fast retrieval
			err := ch.chunkIDToChunkDataPackIDCache.PutWithLockTx(
				lctx, protocolDBBatch, c.ChunkID, chunkDataPackID)
			if err != nil {
				return fmt.Errorf("cannot index stored chunk data pack ID by chunk ID: %w", err)
			}
		}

		return nil
	}

	// Returned Functor: when invoked, will add the deferred storage operations to the provided ReaderBatchWriter
	// NOTE: until this functor is called, only the chunk data packs are stored by their respective IDs.
	return storeChunkDataPacksFunc, nil
}

// BatchRemove remove multiple ChunkDataPacks with the given chunk IDs.
// It performs a two-phase removal:
// 1. First phase: Remove index mappings from ChunkID to chunkDataPackID in the protocol database
// 2. Second phase: Remove chunk data packs (StoredChunkDataPack) by its hash (chunkDataPackID) in chunk data pack database.
// Note: it does not remove the collection referred by the chunk data pack.
// This method is useful for the rollback execution tool to batch remove chunk data packs associated with a set of blocks.
// No errors are expected during normal operation, even if no entries are matched.
func (ch *ChunkDataPacks) BatchRemove(
	chunkIDs []flow.Identifier,
	protocolDBBatch storage.ReaderBatchWriter,
) ([]flow.Identifier, error) {
	// First, collect all stored chunk data pack IDs that need to be removed
	var chunkDataPackIDs []flow.Identifier
	for _, chunkID := range chunkIDs {
		chunkDataPackID, err := ch.chunkIDToChunkDataPackIDCache.Get(protocolDBBatch.GlobalReader(), chunkID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				// If we can't find the stored chunk data pack ID, continue with other removals
				// This handles the case where the chunk data pack was never properly stored
				continue
			}

			return nil, fmt.Errorf("cannot retrieve stored chunk data pack ID for chunk %x: %w", chunkID, err)
		}
		chunkDataPackIDs = append(chunkDataPackIDs, chunkDataPackID)
	}

	// Remove the stored chunk data packs
	if len(chunkDataPackIDs) > 0 {
		storage.OnCommitSucceed(protocolDBBatch, func() {
			// no errors expected during normal operation, even if no entries exist with the given IDs
			err := ch.stored.Remove(chunkDataPackIDs)
			if err != nil {
				log.Fatal().Msgf("cannot remove stored chunk data packs: %v", err)
			}
		})
	}

	// Remove the chunk data pack ID mappings and update cache
	for _, chunkID := range chunkIDs {
		err := ch.chunkIDToChunkDataPackIDCache.RemoveTx(protocolDBBatch, chunkID)
		if err != nil {
			return nil, err
		}
	}
	return chunkDataPackIDs, nil
}

// BatchRemoveChunkDataPacksOnly removes multiple ChunkDataPacks with the given chunk IDs from chunk data pack database only.
// It does not remove the index mappings from ChunkID to chunkDataPackID in the protocol database.
// This method is useful for the runtime chunk data pack pruner to batch remove chunk data packs associated with a set of blocks.
// CAUTION: the chunk data pack batch is for chunk data pack database only, DO NOT pass a batch writer for protocol database.
// No errors are expected during normal operation, even if no entries are matched.
func (ch *ChunkDataPacks) BatchRemoveChunkDataPacksOnly(chunkIDs []flow.Identifier, chunkDataPackBatch storage.ReaderBatchWriter) error {
	// First, collect all stored chunk data pack IDs that need to be removed
	var chunkDataPackIDs []flow.Identifier
	for _, chunkID := range chunkIDs {
		chunkDataPackID, err := ch.chunkIDToChunkDataPackIDCache.Get(ch.protocolDB.Reader(), chunkID) // remove from cache optimistically
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				// If we can't find the stored chunk data pack ID, continue with other removals
				// This handles the case where the chunk data pack was never properly stored
				continue
			}

			return fmt.Errorf("cannot retrieve stored chunk data pack ID for chunk %x: %w", chunkID, err)
		}
		chunkDataPackIDs = append(chunkDataPackIDs, chunkDataPackID)
	}

	// Remove the stored chunk data packs
	if len(chunkDataPackIDs) > 0 {
		err := ch.stored.BatchRemove(chunkDataPackIDs, chunkDataPackBatch)
		if err != nil {
			return fmt.Errorf("cannot remove stored chunk data packs: %w", err)
		}
	}

	// The chunk data pack pruner only removes the stored chunk data packs (in ch.stored).
	// It does not delete the corresponding index mappings from the protocol database.
	// These mappings should be cleaned up by the protocol DB pruner, which will be implemented later.
	return nil
}

// ByChunkID returns the chunk data for the given chunk ID.
// It returns [storage.ErrNotFound] if no entry exists for the given chunk ID.
func (ch *ChunkDataPacks) ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error) {
	// First, retrieve the chunk data pack ID (using cache if available)
	chunkDataPackID, err := ch.chunkIDToChunkDataPackIDCache.Get(ch.protocolDB.Reader(), chunkID)
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve stored chunk data pack ID for chunk %x: %w", chunkID, err)
	}

	// Then retrieve the reduced representation of the chunk data pack via its ID.
	schdp, err := ch.stored.ByID(chunkDataPackID)
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve stored chunk data pack %x for chunk %x: %w", chunkDataPackID, chunkID, err)
	}

	var collection *flow.Collection // nil by default, which only represents system chunk
	if schdp.CollectionID != flow.ZeroID {
		collection, err = ch.collections.ByID(schdp.CollectionID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve collection (id: %x) for stored chunk data pack: %w", schdp.CollectionID, err)
		}
	}

	return flow.NewChunkDataPack(
		schdp.ChunkID,
		schdp.StartState,
		schdp.Proof,
		collection,
		schdp.ExecutionDataRoot,
	), nil
}
