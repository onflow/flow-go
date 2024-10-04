package storage

import (
	"errors"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/evm/types"
)

type ReadOnlyStorage struct {
	snapshot types.BackendStorageSnapshot
}

var _ types.BackendStorage = &ReadOnlyStorage{}

func NewReadOnlyStorage(snapshot types.BackendStorageSnapshot) *ReadOnlyStorage {
	return &ReadOnlyStorage{
		snapshot,
	}
}

// GetValue reads a register value
func (s *ReadOnlyStorage) GetValue(owner []byte, key []byte) ([]byte, error) {
	return s.snapshot.GetValue(owner, key)
}

// SetValue is a noop
func (s *ReadOnlyStorage) SetValue(owner, key, value []byte) error {
	return nil
}

// ValueExists checks if a register exists
func (s *ReadOnlyStorage) ValueExists(owner []byte, key []byte) (bool, error) {
	val, err := s.snapshot.GetValue(owner, key)
	return len(val) > 0, err
}

// AllocateSlabIndex returns error if called
func (s *ReadOnlyStorage) AllocateSlabIndex(owner []byte) (atree.SlabIndex, error) {
	return atree.SlabIndex{}, errors.New("unexpected call received")
}
