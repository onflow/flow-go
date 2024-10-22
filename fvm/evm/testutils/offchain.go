package testutils

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/evm/offchain/storage"
	"github.com/onflow/flow-go/fvm/evm/types"
)

// TestStorageProvider constructs a new
// storage provider that only provides
// storage for an specific height
type TestStorageProvider struct {
	storage types.BackendStorage
	height  uint64
}

var _ types.StorageProvider = &TestStorageProvider{}

// NewTestStorageProvider constructs a new TestStorageProvider
func NewTestStorageProvider(
	initSnapshot types.BackendStorageSnapshot,
	height uint64,
) *TestStorageProvider {
	return &TestStorageProvider{
		storage: storage.NewEphemeralStorage(storage.NewReadOnlyStorage(initSnapshot)),
		height:  height,
	}
}

// GetSnapshotAt returns the snapshot at specific block height
func (sp *TestStorageProvider) GetSnapshotAt(height uint64) (types.BackendStorageSnapshot, error) {
	if height != sp.height {
		return nil, fmt.Errorf("storage for the given height (%d) is not available", height)
	}
	return sp.storage, nil
}
