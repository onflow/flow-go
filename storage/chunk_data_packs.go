package storage

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
)

// ChunkDataPacks represents persistent storage for chunk data packs.
type ChunkDataPacks interface {

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
	//   - func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error: Function for updating the chunk data pack database
	//     and protocol database within a batch update. This function returns [storage.ErrDataMismatch] when an _different_
	//     chunk data pack ID for the same chunk ID has already been stored.
	//     The caller must acquire [storage.LockInsertChunkDataPack] and hold it until the database write has been committed.
	//   - error: No error should be returned during normal operation. Any error indicates a failure in the first phase.
	Store(cs []*flow.ChunkDataPack) (func(lctx lockctx.Proof, rw ReaderBatchWriter) error, error)

	// ByChunkID returns the chunk data for the given chunk ID.
	// It returns [storage.ErrNotFound] if no entry exists for the given chunk ID.
	ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error)

	// BatchRemove schedules all ChunkDataPacks with the given IDs to be deleted from the databases,
	// part of the provided write batches. Unknown IDs are silently ignored.
	// It performs a two-phase removal:
	// 1. First phase: Remove index mappings from ChunkID to storedChunkDataPackID in the protocol database
	// 2. Second phase: Remove chunk data packs (StoredChunkDataPack) by its hash (storedChunkDataPackID) in chunk data pack database.
	// Note: it does not remove the collection referred by the chunk data pack.
	// This method is useful for the rollback execution tool to batch remove chunk data packs associated with a set of blocks.
	// No errors are expected during normal operation, even if no entries are matched.
	BatchRemove(chunkIDs []flow.Identifier, protocolDBBatch ReaderBatchWriter, chunkDataPackDBBatch ReaderBatchWriter) error

	// BatchRemoveStoredChunkDataPacksOnly removes multiple ChunkDataPacks with the given chunk IDs from chunk data pack database only.
	// It does not remove the index mappings from ChunkID to storedChunkDataPackID in the protocol database.
	// This method is useful for the runtime chunk data pack pruner to batch remove chunk data packs associated with a set of blocks.
	// No errors are expected during normal operation, even if no entries are matched.
	BatchRemoveStoredChunkDataPacksOnly(chunkIDs []flow.Identifier, chunkDataPackDBBatch ReaderBatchWriter) error
}
