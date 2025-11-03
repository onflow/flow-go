package storage

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
)

// ChunkDataPacks represents persistent storage for chunk data packs.
type ChunkDataPacks interface {

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
	Store(cs []*flow.ChunkDataPack) (func(lctx lockctx.Proof, protocolDBBatch ReaderBatchWriter) error, error)

	// ByChunkID returns the chunk data for the given chunk ID.
	// It returns [storage.ErrNotFound] if no entry exists for the given chunk ID.
	ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error)

	// BatchRemove schedules all ChunkDataPacks with the given IDs to be deleted from the databases,
	// part of the provided write batches. Unknown IDs are silently ignored.
	// It returns the list of chunk data pack IDs (chunkDataPackID) that were scheduled for removal from the chunk data pack database.
	// It performs a two-phase removal:
	// 1. First phase: Remove index mappings from ChunkID to chunkDataPackID in the protocol database
	// 2. Second phase: Remove chunk data packs (StoredChunkDataPack) by its hash (chunkDataPackID) in chunk data pack database.
	//  This phase is deferred until the caller of BatchRemove invokes the returned functor.
	//
	// Note: it does not remove the collection referred by the chunk data pack.
	// This method is useful for the rollback execution tool to batch remove chunk data packs associated with a set of blocks.
	// No errors are expected during normal operation, even if no entries are matched.
	BatchRemove(chunkIDs []flow.Identifier, rw ReaderBatchWriter) (chunkDataPackIDs []flow.Identifier, err error)

	// BatchRemoveChunkDataPacksOnly removes multiple ChunkDataPacks with the given chunk IDs from chunk data pack database only.
	// It does not remove the index mappings from ChunkID to chunkDataPackID in the protocol database.
	// This method is useful for the runtime chunk data pack pruner to batch remove chunk data packs associated with a set of blocks.
	// CAUTION: the chunk data pack batch is for chunk data pack database only, DO NOT pass a batch writer for protocol database.
	// No errors are expected during normal operation, even if no entries are matched.
	BatchRemoveChunkDataPacksOnly(chunkIDs []flow.Identifier, chunkDataPackBatch ReaderBatchWriter) error
}
