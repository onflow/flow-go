package storage

import (
	"bytes"

	"github.com/onflow/flow-go/model/flow"
)

// ChunkDataPacks represents persistent storage for chunk data packs.
type StoredChunkDataPacks interface {
	// StoreChunkDataPacks stores multiple StoredChunkDataPacks cs in a batch.
	// It returns the IDs of the stored chunk data packs.
	// No error are expected during normal operation.
	StoreChunkDataPacks(cs []*StoredChunkDataPack) ([]flow.Identifier, error)

	// ByID returns the StoredChunkDataPack for the given ID.
	// It returns [storage.ErrNotFound] if no entry exists for the given ID.
	ByID(id flow.Identifier) (*StoredChunkDataPack, error)

	// Remove removes multiple ChunkDataPacks cs keyed by their StoredChunkDataPack IDs in a batch.
	// No errors are expected during normal operation.
	Remove(cs []flow.Identifier) error

	BatchRemove(cs []flow.Identifier, batch ReaderBatchWriter) error
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

func ToStoredChunkDataPacks(cs []*flow.ChunkDataPack) []*StoredChunkDataPack {
	scs := make([]*StoredChunkDataPack, 0, len(cs))
	for _, c := range cs {
		scs = append(scs, ToStoredChunkDataPack(c))
	}
	return scs
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

func (c StoredChunkDataPack) ID() flow.Identifier {
	return flow.MakeID(c)
}
