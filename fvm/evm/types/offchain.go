package types

import (
	"github.com/onflow/flow-go/model/flow"
)

// BackendStorageSnapshot provides a read only view of registers
type BackendStorageSnapshot interface {
	GetValue(owner []byte, key []byte) ([]byte, error)
}

// StorageProvider provides access to storage at
// specific time point in history of the EVM chain
type StorageProvider interface {
	// GetSnapshotAt returns a readonly snapshot of storage
	// at specific block (start state of the block before executing transactions)
	GetSnapshotAt(evmBlockHeight uint64) (BackendStorageSnapshot, error)
}

// BlockSnapshot provides access to the block information
// at specific block height
type BlockSnapshot interface {
	// BlockContext constructs and returns the block context for the block
	//
	// Warning! the block hash provider on this one has to return empty
	// for the current block to stay compatible with how on-chain EVM
	// behaves. so if we are on block 10, and we query for the block hash on block
	// 10 it should return empty hash.
	BlockContext() (BlockContext, error)
}

type BlockSnapshotProvider interface {
	// GetSnapshotAt returns a readonly snapshot of block given evm block height
	GetSnapshotAt(evmBlockHeight uint64) (BlockSnapshot, error)
}

// ReplayResultCollector collects results of replay a block
type ReplayResultCollector interface {
	// StorageRegisterUpdates returns the set of register changes
	// (only the EVM-related registers)
	StorageRegisterUpdates() map[flow.RegisterID]flow.RegisterValue
}
