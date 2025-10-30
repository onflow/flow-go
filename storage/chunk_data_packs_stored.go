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
	// It returns the chunk data pack IDs
	// No error returns are expected during normal operation.
	StoreChunkDataPacks(cs []*StoredChunkDataPack) ([]flow.Identifier, error)

	// ByID returns the StoredChunkDataPack for the given ID.
	// It returns [storage.ErrNotFound] if no entry exists for the given ID.
	ByID(id flow.Identifier) (*StoredChunkDataPack, error)

	// Remove removes multiple ChunkDataPacks cs keyed by their IDs in a batch.
	// No error returns are expected during normal operation, even if none of the referenced objects exist in storage.
	Remove(chunkDataPackIDs []flow.Identifier) error

	// BatchRemove removes multiple ChunkDataPacks with the given IDs from storage as part of the provided write batch.
	// No error returns are expected during normal operation, even if no entries are matched.
	BatchRemove(chunkDataPackIDs []flow.Identifier, rw ReaderBatchWriter) error
}

// StoredChunkDataPack is an in-storage representation of chunk data pack. Its prime difference is instead of an
// actual collection, it keeps a collection ID hence relying on maintaining the collection on a secondary storage.
// Note, StoredChunkDataPack.ID() is the same as ChunkDataPack.ID()
//
//structwrite:immutable - mutations allowed only within the constructor
type StoredChunkDataPack struct {
	ChunkID           flow.Identifier
	StartState        flow.StateCommitment
	Proof             flow.StorageProof
	CollectionID      flow.Identifier // flow.ZeroID for system chunks
	ExecutionDataRoot flow.BlockExecutionDataRoot
}

// NewStoredChunkDataPack instantiates an "immutable"  [StoredChunkDataPack].
// The `collectionID` field is set to [flow.ZeroID] for system chunks.
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

// IsSystemChunk returns true if this chunk data pack is for a system chunk.
func (s *StoredChunkDataPack) IsSystemChunk() bool {
	return s.CollectionID == flow.ZeroID
}

// ToStoredChunkDataPack converts the given Chunk Data Pack to its reduced representation.
// (Collections are stored separately and don't need to be included again here).
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

// ToStoredChunkDataPacks converts the given Chunk Data Packs to their reduced representation.
// (Collections are stored separately and don't need to be included again here).
func ToStoredChunkDataPacks(cs []*flow.ChunkDataPack) []*StoredChunkDataPack {
	scs := make([]*StoredChunkDataPack, 0, len(cs))
	for _, c := range cs {
		scs = append(scs, ToStoredChunkDataPack(c))
	}
	return scs
}

// Equals compares two StoredChunkDataPack for equality.
// It returns (true, "") if they are equal, otherwise (false, reason) where reason is the first
// found reason for the mismatch.
func (c StoredChunkDataPack) Equals(other StoredChunkDataPack) (equal bool, diffReason string) {
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

// ID returns the identifier of the chunk data pack, which is derived from its contents.
// Note, StoredChunkDataPack.ID() is the same as ChunkDataPack.ID()
func (c StoredChunkDataPack) ID() flow.Identifier {
	return flow.NewChunkDataPackHeader(c.ChunkID, c.StartState, flow.MakeID(c.Proof), c.CollectionID, c.ExecutionDataRoot).ID()
}
