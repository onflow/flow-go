package execution

import (
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/finalizedreader"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble"
)

// RegisterStore is the interface for register store
// see implementation in engine/execution/storehouse/register_store.go
type RegisterStore interface {
	// GetRegister first try to get the register from InMemoryRegisterStore, then OnDiskRegisterStore
	// It returns:
	//  - (value, nil) if the register value is found at the given block
	//  - (nil, nil) if the register is not found
	//  - (nil, storage.ErrHeightNotIndexed) if the height is below the first height that is indexed.
	//  - (nil, storehouse.ErrNotExecuted) if the block is not executed yet
	//  - (nil, storehouse.ErrNotExecuted) if the block is conflicting iwth finalized block
	//  - (nil, err) for any other exceptions
	GetRegister(height uint64, blockID flow.Identifier, register flow.RegisterID) (flow.RegisterValue, error)

	// SaveRegisters saves to InMemoryRegisterStore first, then trigger the same check as OnBlockFinalized
	// Depend on InMemoryRegisterStore.SaveRegisters
	// It returns:
	// - nil if the registers are saved successfully
	// - exception is the block is above the pruned height but does not connect to the pruned height (conflicting block).
	// - exception if the block is below the pruned height
	// - exception if the save block is saved again
	// - exception for any other exception
	SaveRegisters(header *flow.Header, registers flow.RegisterEntries) error

	// Depend on FinalizedReader's FinalizedBlockIDAtHeight
	// Depend on ExecutedFinalizedWAL.Append
	// Depend on OnDiskRegisterStore.SaveRegisters
	// OnBlockFinalized trigger the check of whether a block at the next height becomes finalized and executed.
	// Note: This is a blocking call
	// the next height is the existing finalized and executed block's height + 1.
	// If a block at next height becomes finalized and executed, then:
	// 1. write the registers to write ahead logs
	// 2. save the registers of the block to OnDiskRegisterStore
	// 3. prune the height in InMemoryRegisterStore
	// any error returned are exception
	OnBlockFinalized() error

	// LastFinalizedAndExecutedHeight returns the height of the last finalized and executed block,
	// which has been saved in OnDiskRegisterStore
	LastFinalizedAndExecutedHeight() uint64

	// IsBlockExecuted returns whether the given block is executed.
	// If a block is not executed, it does not distinguish whether the block exists or not.
	// It returns:
	// - (true, nil) if the block is executed, regardless of whether the registers of the block is pruned on disk or not
	// - (false, nil) if the block is not executed
	// - (false, exception) if running into any exception
	IsBlockExecuted(height uint64, blockID flow.Identifier) (bool, error)
}

type FinalizedReader interface {
	// FinalizedBlockIDAtHeight returns the block ID of the finalized block at the given height.
	// It return storage.NotFound if the given height has not been finalized yet
	// any other error returned are exceptions
	FinalizedBlockIDAtHeight(height uint64) (flow.Identifier, error)
}

// finalizedreader.FinalizedReader is an implementation of FinalizedReader interface
var _ FinalizedReader = (*finalizedreader.FinalizedReader)(nil)

// see implementation in engine/execution/storehouse/in_memory_register_store.go
type InMemoryRegisterStore interface {
	Prune(finalizedHeight uint64, finalizedBlockID flow.Identifier) error
	PrunedHeight() uint64

	// GetRegister will return the latest updated value of the given register since the pruned height.
	// It returns ErrPruned if the register is unknown or not updated since the pruned height
	// It returns exception if internal index is inconsistent
	GetRegister(height uint64, blockID flow.Identifier, register flow.RegisterID) (flow.RegisterValue, error)
	GetUpdatedRegisters(height uint64, blockID flow.Identifier) (flow.RegisterEntries, error)
	SaveRegisters(
		height uint64,
		blockID flow.Identifier,
		parentID flow.Identifier,
		registers flow.RegisterEntries,
	) error

	IsBlockExecuted(height uint64, blockID flow.Identifier) (bool, error)
}

type OnDiskRegisterStore = storage.RegisterIndex

// pebble.Registers is an implementation of OnDiskRegisterStore interface
var _ OnDiskRegisterStore = (*pebble.Registers)(nil)

type ExecutedFinalizedWAL interface {
	Append(height uint64, registers flow.RegisterEntries) error

	// Latest returns the latest height in the WAL.
	Latest() (uint64, error)

	GetReader(height uint64) WALReader
}

type WALReader interface {
	// Next returns the next height and trie updates in the WAL.
	// It returns EOF when there are no more entries.
	Next() (height uint64, registers flow.RegisterEntries, err error)
}

type ExtendableStorageSnapshot interface {
	snapshot.StorageSnapshot
	Extend(newCommit flow.StateCommitment, updatedRegisters map[flow.RegisterID]flow.RegisterValue) ExtendableStorageSnapshot
	Commitment() flow.StateCommitment
}
