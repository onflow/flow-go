package storage

import (
	"bytes"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// StoredChunkDataPacks represents persistent storage for chunk data packs.
// It works with the reduced representation `StoredChunkDataPack` for chunk data packs,
// where instead of the full collection text, only the collection's hash (ID) is contained.
type StoredChunkDataPacks interface {
	// StoreChunkDataPacks stores multiple StoredChunkDataPacks cs in a batch.
	// It returns the IDs of the stored chunk data packs.
	// No error returns are expected during normal operation.
	StoreChunkDataPacks(cs []*StoredChunkDataPack) ([]flow.Identifier, error)

	// ByID returns the StoredChunkDataPack for the given ID.
	// It returns [storage.ErrNotFound] if no entry exists for the given ID.
	ByID(id flow.Identifier) (*StoredChunkDataPack, error)

	// Remove removes multiple ChunkDataPacks cs keyed by their StoredChunkDataPack IDs in a batch.
	// No errors returns are expected during normal operation.
	Remove(cs []flow.Identifier) error

	// BatchRemove removes multiple StoredChunkDataPacks cs keyed by their IDs in a batch using the provided
	// No errors returns are expected during normal operation, even if no entries are matched.
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

// Equals compares two StoredChunkDataPack for equality.
// The second return value is a string describing the first found mismatch, or empty if they are equal.
func (c StoredChunkDataPack) Equals(other StoredChunkDataPack) (bool, string) {
	if c.ChunkID != other.ChunkID {
		return false, fmt.Errorf("chunk ID mismatch: %s != %s", c.ChunkID, other.ChunkID).Error()
	}
	if c.StartState != other.StartState {
		return false, fmt.Errorf("start state mismatch: %s != %s", c.StartState, other.StartState).Error()
	}
	if !c.ExecutionDataRoot.Equals(other.ExecutionDataRoot) {
		return false, fmt.Sprintf("execution data root mismatch: %s != %s", c.ExecutionDataRoot, other.ExecutionDataRoot)
	}
	if c.SystemChunk != other.SystemChunk {
		return false, fmt.Sprintf("system chunk flag mismatch: %t != %t", c.SystemChunk, other.SystemChunk)
	}
	if !bytes.Equal(c.Proof, other.Proof) {
		return false, "storage proof mismatch"
	}
	if c.CollectionID != other.CollectionID {
		return false, fmt.Sprintf("collection ID mismatch: %s != %s", c.CollectionID, other.CollectionID)
	}
	return true, ""
}

func (c StoredChunkDataPack) ID() flow.Identifier {
	return flow.MakeID(c)
}
