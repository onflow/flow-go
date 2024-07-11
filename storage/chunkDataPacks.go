package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// ChunkDataPacks represents persistent storage for chunk data packs.
type ChunkDataPacks interface {
	Prunable

	// Store stores multiple ChunkDataPacks cs keyed by their ChunkIDs in a batch.
	// No errors are expected during normal operation, but it may return generic error
	Store(cs []*flow.ChunkDataPack) error

	// Remove removes multiple ChunkDataPacks cs keyed by their ChunkIDs in a batch.
	// No errors are expected during normal operation, but it may return generic error
	Remove(cs []flow.Identifier) error

	// ByChunkID returns the chunk data for the given a chunk ID.
	ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error)

	// BatchRemove removes ChunkDataPack c keyed by its ChunkID in provided batch
	// No errors are expected during normal operation, even if no entries are matched.
	// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
	BatchRemove(chunkID flow.Identifier, batch BatchStorage) error
}

// StoredChunkDataPack is an in-storage representation of chunk data pack.
// Its prime difference is instead of an actual collection, it keeps a collection ID hence relying on maintaining
// the collection on a secondary storage.
type StoredChunkDataPack struct {
	ChunkID           flow.Identifier
	StartState        flow.StateCommitment
	Proof             flow.StorageProof
	CollectionID      flow.Identifier
	SystemChunk       bool
	ExecutionDataRoot flow.BlockExecutionDataRoot
}

func ToStoredChunkDataPack(c *flow.ChunkDataPack) *StoredChunkDataPack {
	sc := &StoredChunkDataPack{
		ChunkID:           c.ChunkID,
		StartState:        c.StartState,
		Proof:             c.Proof,
		SystemChunk:       false,
		ExecutionDataRoot: c.ExecutionDataRoot,
	}

	if c.Collection != nil {
		// non system chunk
		sc.CollectionID = c.Collection.ID()
	} else {
		sc.SystemChunk = true
	}

	return sc
}
