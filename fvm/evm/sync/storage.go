package sync

import (
	"fmt"

	"github.com/onflow/atree"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

// StorageProvider provides access to storage at
// specific time point in history of the EVM chain
type StorageProvider interface {
	GetStorageAt(height uint64, txIndex uint64) (types.BackendStorage, error)
	GetStorageByHeight(height uint64) (types.BackendStorage, error)
}

// EphemeralStorage is used for cases that we don't want to
// pass the changes back to the storage. TODO: update me
type EphemeralStorage struct {
	parent types.BackendStorage
	deltas map[flow.RegisterID]flow.RegisterValue
}

// NewEphemeralStorage constructs a new EphemeralStorage
func NewEphemeralStorage(parent types.BackendStorage) *EphemeralStorage {
	deltas := make(map[flow.RegisterID]flow.RegisterValue)
	return &EphemeralStorage{parent: parent, deltas: deltas}
}

var _ types.BackendStorage = &EphemeralStorage{}

func (s *EphemeralStorage) GetValue(owner []byte, key []byte) ([]byte, error) {
	// check delta first
	ret, found := s.deltas[RegisterID(owner, key)]
	if found {
		return ret, nil
	}
	return s.parent.GetValue(owner, key)
}

func (s *EphemeralStorage) SetValue(owner, key, value []byte) error {
	s.deltas[RegisterID(owner, key)] = value
	return nil
}

func (s *EphemeralStorage) ValueExists(owner []byte, key []byte) (bool, error) {
	ret, err := s.GetValue(owner, key)
	return len(ret) > 0, err
}

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

// Warning! this commit won't sort the updates before commit
// not intended to be used for on-chain operations
func (s *EphemeralStorage) Commit() error {
	var err error
	for k, v := range s.deltas {
		err = s.parent.SetValue([]byte(k.Owner), []byte(k.Key), v)
		if err != nil {
			return err
		}
	}
	return nil
}

func RegisterID(owner []byte, key []byte) flow.RegisterID {
	return flow.NewRegisterID(flow.BytesToAddress(owner), string(key))
}
