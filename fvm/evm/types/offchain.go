package types

import "github.com/onflow/flow-go/model/flow"

// BackendStorageSnapshot provides a read only view of registers
type BackendStorageSnapshot interface {
	GetValue(owner []byte, key []byte) ([]byte, error)
}

// StorageProvider provides access to storage at
// specific time point in history of the EVM chain
type StorageProvider interface {
	// GetSnapshotAt returns a readonly snapshot of storage
	// at specific block (start state of the block before executing transactions)
	GetSnapshotAt(height uint64) (BackendStorageSnapshot, error)
}

// ReplayResults is the result of replaying transactions
type ReplayResults interface {
	// StorageRegisterUpdates returns the set of register changes
	// (only the EVM-related registers)
	StorageRegisterUpdates() map[flow.RegisterID]flow.RegisterValue
}
