package storehouse

import (
	"fmt"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

var _ execution.ExtendableStorageSnapshot = (*ExecutingBlockSnapshot)(nil)

// ExecutingBlockSnapshot is a snapshot of the storage at an executed collection.
// It starts with a storage snapshot at the end of previous block,
// The register updates at the executed collection at baseHeight + 1 are cached in
// a map, such that retrieving register values at the snapshot will first check
// the cache, and then the storage.
type ExecutingBlockSnapshot struct {
	// the snapshot at the end of previous block
	previous snapshot.StorageSnapshot

	commitment      flow.StateCommitment
	registerUpdates map[flow.RegisterID]flow.RegisterValue
}

// create a new storage snapshot for an executed collection
// at the base block at height h - 1
func NewExecutingBlockSnapshot(
	previous snapshot.StorageSnapshot,
	// the statecommitment of a block at height h
	commitment flow.StateCommitment,
) *ExecutingBlockSnapshot {
	return &ExecutingBlockSnapshot{
		previous:        previous,
		commitment:      commitment,
		registerUpdates: make(map[flow.RegisterID]flow.RegisterValue),
	}
}

// Get returns the register value at the snapshot.
func (s *ExecutingBlockSnapshot) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	// get from latest updates first
	value, ok := s.getFromUpdates(id)
	if ok {
		return value, nil
	}

	// get from BlockEndStateSnapshot at previous block
	value, err := s.previous.Get(id)
	return value, err
}

func (s *ExecutingBlockSnapshot) getFromUpdates(id flow.RegisterID) (flow.RegisterValue, bool) {
	value, ok := s.registerUpdates[id]
	return value, ok
}

// Extend returns a new storage snapshot at the same block but but for a different state commitment,
// which contains the given registerUpdates
// Usually it's used to create a new storage snapshot at the next executed collection.
// The registerUpdates contains the register updates at the executed collection.
func (s *ExecutingBlockSnapshot) Extend(newCommit flow.StateCommitment, updates map[flow.RegisterID]flow.RegisterValue) execution.ExtendableStorageSnapshot {
	if len(updates) == 0 {
		fmt.Println("=========extending")
		return s
	}

	return &ExecutingBlockSnapshot{
		previous:        s,
		commitment:      newCommit,
		registerUpdates: updates,
	}
}

func (s *ExecutingBlockSnapshot) Commitment() flow.StateCommitment {
	return s.commitment
}
