package storage

import (
	"fmt"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

// EphemeralStorage holds on to register changes instead of applying them directly to
// the provided backend storage. It can be used for dry running transaction/calls
// or batching updates for atomic operations.
type EphemeralStorage struct {
	parent types.BackendStorage
	deltas map[flow.RegisterID]flow.RegisterValue
}

// NewEphemeralStorage constructs a new EphemeralStorage
func NewEphemeralStorage(parent types.BackendStorage) *EphemeralStorage {
	return &EphemeralStorage{
		parent: parent,
		deltas: make(map[flow.RegisterID]flow.RegisterValue),
	}
}

var _ types.BackendStorage = &EphemeralStorage{}

// GetValue reads a register value
func (s *EphemeralStorage) GetValue(owner []byte, key []byte) ([]byte, error) {
	// check delta first
	ret, found := s.deltas[RegisterID(owner, key)]
	if found {
		return ret, nil
	}
	return s.parent.GetValue(owner, key)
}

// SetValue sets a register value
func (s *EphemeralStorage) SetValue(owner, key, value []byte) error {
	s.deltas[RegisterID(owner, key)] = value
	return nil
}

// ValueExists checks if a register exists
func (s *EphemeralStorage) ValueExists(owner []byte, key []byte) (bool, error) {
	ret, err := s.GetValue(owner, key)
	return len(ret) > 0, err
}

// AllocateSlabIndex allocates an slab index based on the given owner
func (s *EphemeralStorage) AllocateSlabIndex(owner []byte) (atree.SlabIndex, error) {
	statusBytes, err := s.GetValue(owner, []byte(flow.AccountStatusKey))
	if err != nil {
		return atree.SlabIndex{}, err
	}
	if len(statusBytes) == 0 {
		return atree.SlabIndex{}, fmt.Errorf("state for account not found")
	}

	status, err := environment.AccountStatusFromBytes(statusBytes)
	if err != nil {
		return atree.SlabIndex{}, err
	}

	// get and increment the index
	index := status.SlabIndex()
	newIndexBytes := index.Next()

	// update the storageIndex bytes
	status.SetStorageIndex(newIndexBytes)
	err = s.SetValue(owner, []byte(flow.AccountStatusKey), status.ToBytes())
	if err != nil {
		return atree.SlabIndex{}, err
	}
	return index, nil
}

// StorageRegisterUpdates returns a map of register updates
func (s *EphemeralStorage) StorageRegisterUpdates() map[flow.RegisterID]flow.RegisterValue {
	return s.deltas
}

// Commit commits changes from the delta to the underlying backend storage
//
// Warning! commit method doesn't sort the updates before sending the the backend
// so it's not intended to be used for on-chain operations
func (s *EphemeralStorage) Commit() error {
	var err error
	for k, v := range s.deltas {
		err = s.parent.SetValue([]byte(k.Owner), []byte(k.Key), v)
		if err != nil {
			return err
		}
	}
	// reset delta
	s.deltas = make(map[flow.RegisterID]flow.RegisterValue)
	return nil
}

// RegisterID creates a RegisterID from owner and key
func RegisterID(owner []byte, key []byte) flow.RegisterID {
	return flow.NewRegisterID(flow.BytesToAddress(owner), string(key))
}
