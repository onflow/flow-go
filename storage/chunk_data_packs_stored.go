package storage

import (
	"bytes"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// StoredChunkDataPacks represents persistent storage for chunk data packs.
// It works with the reduced representation `StoredChunkDataPack` for chunk data packs,
// where instead of the full collection data, only the collection's hash (ID) is contained.
type StoredChunkDataPacks interface {
	// StoreChunkDataPacks stores multiple StoredChunkDataPacks cs in a batch.
	// It returns the IDs of the stored chunk data packs.
	// No error returns are expected during normal operation.
	StoreChunkDataPacks(cs []*StoredChunkDataPack) ([]flow.Identifier, error)

	// ByID returns the StoredChunkDataPack for the given ID.
	// It returns [storage.ErrNotFound] if no entry exists for the given ID.
	ByID(id flow.Identifier) (*StoredChunkDataPack, error)

	// Remove removes multiple ChunkDataPacks cs keyed by their StoredChunkDataPack IDs in a batch.
	// No error returns are expected during normal operation, even if none of the referenced objects exist in storage.
	Remove(cs []flow.Identifier) error

	// BatchRemove removes multiple ChunkDataPacks with the given IDs from storage as part of the provided write batch.
	// No error returns are expected during normal operation, even if no entries are matched.
	BatchRemove(chunkDataPackIDs []flow.Identifier, rw ReaderBatchWriter) error
}

// StoredChunkDataPack is an in-storage representation of chunk data pack.
// Its prime difference is instead of an actual collection, it keeps a collection ID hence relying on maintaining
// the collection on a secondary storage.
//
//structwrite:immutable - mutations allowed only within the constructor
type StoredChunkDataPack struct {
	ChunkID           flow.Identifier
	StartState        flow.StateCommitment
	Proof             flow.StorageProof
	CollectionID      flow.Identifier
	ExecutionDataRoot flow.BlockExecutionDataRoot
}

func NewStoredChunkDataPack(
	chunkID flow.Identifier,
	startState flow.StateCommitment,
	proof flow.StorageProof,
	collectionID flow.Identifier,
	executionDataRoot flow.BlockExecutionDataRoot,
) *StoredChunkDataPack {
	return &StoredChunkDataPack{
		ChunkID:           chunkID,
		StartState:        startState,
		Proof:             proof,
		CollectionID:      collectionID,
		ExecutionDataRoot: executionDataRoot,
	}
}

func (s *StoredChunkDataPack) IsSystemChunk() bool {
	return s.CollectionID == flow.ZeroID
}

func ToStoredChunkDataPack(c *flow.ChunkDataPack) *StoredChunkDataPack {
	collectionID := flow.ZeroID
	if c.Collection != nil {
		collectionID = c.Collection.ID()
	}

	return NewStoredChunkDataPack(
		c.ChunkID,
		c.StartState,
		c.Proof,
		collectionID,
		c.ExecutionDataRoot,
	)
}

func ToStoredChunkDataPacks(cs []*flow.ChunkDataPack) []*StoredChunkDataPack { // ToStoredChunkDataPack converts the given ChunkDataPacks to their reduced representation,
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
	if !bytes.Equal(c.Proof, other.Proof) {
		return false, "storage proof mismatch"
	}
	if c.CollectionID != other.CollectionID {
		return false, fmt.Sprintf("collection ID mismatch: %s != %s", c.CollectionID, other.CollectionID)
	}
	return true, ""
}

func (c StoredChunkDataPack) ID() flow.Identifier {
	return flow.NewChunkDataPackHeader(c.ChunkID, c.StartState, flow.MakeID(c.Proof), c.CollectionID, c.ExecutionDataRoot).ID()
}
