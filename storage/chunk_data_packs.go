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
	//   - func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error: Function to index the chunk id with
	// 		 chunk data pack hash within batch update to store along with other execution data into protocol database,
	//     this function might return [storage.ErrDataMismatch] when an existing chunk data pack ID is found for
	//     the same chunk ID, and is different from the one being stored.
	//     the caller must acquire [storage.LockInsertChunkDataPack] and hold it until the database write has been committed.
	//   - error: No error should be returned during normal operation. Any error indicates a failure in the first phase.
	Store(cs []*flow.ChunkDataPack) (func(lctx lockctx.Proof, rw ReaderBatchWriter) error, error)

	// ByChunkID returns the chunk data for the given chunk ID.
	// It returns [storage.ErrNotFound] if no entry exists for the given chunk ID.
	ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error)

	// BatchRemove remove multiple ChunkDataPacks with the given chunk IDs (in a batch).
	// Note: it does not remove the collection referred by the chunk data pack.
	// No errors are expected during normal operation, even if no entries are matched.
	BatchRemove(chunkIDs []flow.Identifier, batch ReaderBatchWriter) error
}
