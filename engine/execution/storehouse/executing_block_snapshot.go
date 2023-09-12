package storehouse

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// TODO(leo): move it to upper level
type ExtendableStorageSnapshot interface {
	snapshot.StorageSnapshot
	Extend(newCommit flow.StateCommitment, updatedRegisters flow.RegisterEntries) (ExtendableStorageSnapshot, error)
	Commitment() flow.StateCommitment
}

var _ ExtendableStorageSnapshot = (*ExecutingBlockSnapshot)(nil)

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
// which contains the given trieUpdate
// Usually it's used to create a new storage snapshot at the next executed collection.
// The trieUpdate contains the register updates at the executed collection.
func (s *ExecutingBlockSnapshot) Extend(newCommit flow.StateCommitment, updatedRegisters flow.RegisterEntries) (ExtendableStorageSnapshot, error) {
	updates := make(map[flow.RegisterID]flow.RegisterValue)

	// add the old updates
	// overwrite with new updates
	for _, entry := range updatedRegisters {
		updates[entry.Key] = entry.Value
	}

	return &ExecutingBlockSnapshot{
		previous:        s.previous,
		commitment:      newCommit,
		registerUpdates: updates,
	}, nil
}

func (s *ExecutingBlockSnapshot) Commitment() flow.StateCommitment {
	return s.commitment
}

// TODO(leo): move it, copied from engine/execution/state/state.go to prevent cycle import
const (
	KeyPartOwner = uint16(0)
	// @deprecated - controller was used only by the very first
	// version of cadence for access controll which was retired later on
	// KeyPartController = uint16(1)
	KeyPartKey = uint16(2)
)

// TODO(leo): move it
func KeyToRegisterID(key ledger.Key) (flow.RegisterID, error) {
	if len(key.KeyParts) != 2 ||
		key.KeyParts[0].Type != KeyPartOwner ||
		key.KeyParts[1].Type != KeyPartKey {
		return flow.RegisterID{}, fmt.Errorf("key not in expected format %s", key.String())
	}

	return flow.NewRegisterID(
		string(key.KeyParts[0].Value),
		string(key.KeyParts[1].Value),
	), nil
}

// TODO(leo): move it
func PayloadToRegister(payload *ledger.Payload) (flow.RegisterID, flow.RegisterValue, error) {
	key, err := payload.Key()
	if err != nil {
		return flow.RegisterID{}, flow.RegisterValue{}, fmt.Errorf("could not parse register key from payload: %w", err)
	}
	regID, err := KeyToRegisterID(key)
	if err != nil {
		return flow.RegisterID{}, flow.RegisterValue{}, fmt.Errorf("could not parse register key from payload: %w", err)
	}

	return regID, payload.Value(), nil

}
