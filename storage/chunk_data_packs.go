package storage

import (
	"bytes"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
)

// ChunkDataPacks represents persistent storage for chunk data packs.
type ChunkDataPacks interface {

	// Store stores multiple ChunkDataPacks cs keyed by their ChunkIDs in a batch.
	// No errors are expected during normal operation, but it may return generic error
	StoreByChunkID(lctx lockctx.Proof, cs []*flow.ChunkDataPack) error

	// Remove removes multiple ChunkDataPacks cs keyed by their ChunkIDs in a batch.
	// No errors are expected during normal operation, but it may return generic error
	Remove(cs []flow.Identifier) error

	// ByChunkID returns the chunk data for the given a chunk ID.
	ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error)

	// BatchRemove removes ChunkDataPack c keyed by its ChunkID in provided batch
	// No errors are expected during normal operation, even if no entries are matched.
	// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
	BatchRemove(chunkID flow.Identifier, batch ReaderBatchWriter) error
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

func (c StoredChunkDataPack) Equals(other StoredChunkDataPack) error {
	if c.ChunkID != other.ChunkID {
		return ErrDataMismatch
	}
	if c.StartState != other.StartState {
		return ErrDataMismatch
	}
	if !c.ExecutionDataRoot.Equals(other.ExecutionDataRoot) {
		return ErrDataMismatch
	}
	if c.SystemChunk != other.SystemChunk {
		return ErrDataMismatch
	}
	if !bytes.Equal(c.Proof, other.Proof) {
		return ErrDataMismatch
	}
	if c.CollectionID != other.CollectionID {
		return ErrDataMismatch
	}
	return nil
}
