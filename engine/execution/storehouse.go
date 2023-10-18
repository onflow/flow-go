package execution

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// RegisterStore is the interface for register store
// see implementation in engine/execution/storehouse/register_store.go
type RegisterStore interface {
	// GetRegister first try to get the register from InMemoryRegisterStore, then OnDiskRegisterStore
	GetRegister(height uint64, blockID flow.Identifier, register flow.RegisterID) (flow.RegisterValue, error)

	// SaveRegisters saves to InMemoryRegisterStore first, then trigger the same check as OnBlockFinalized
	// Depend on InMemoryRegisterStore.SaveRegisters
	SaveRegisters(header *flow.Header, registers []flow.RegisterEntry) error

	// Depend on FinalizedReader's GetFinalizedBlockIDAtHeight
	// Depend on ExecutedFinalizedWAL.Append
	// Depend on OnDiskRegisterStore.SaveRegisters
	// OnBlockFinalized trigger the check of whether a block at the next height becomes finalized and executed.
	// the next height is the existing finalized and executed block's height + 1.
	// If a block at next height becomes finalized and executed, then:
	// 1. write the registers to write ahead logs
	// 2. save the registers of the block to OnDiskRegisterStore
	// 3. prune the height in InMemoryRegisterStore
	OnBlockFinalized() error

	// FinalizedAndExecutedHeight returns the height of the last finalized and executed block,
	// which has been saved in OnDiskRegisterStore
	FinalizedAndExecutedHeight() uint64

	// IsBlockExecuted returns whether the given block is executed.
	// If a block is not executed, it does not distinguish whether the block exists or not.
	IsBlockExecuted(height uint64, blockID flow.Identifier) (bool, error)
}

type FinalizedReader interface {
	GetFinalizedBlockIDAtHeight(height uint64) (flow.Identifier, error)
}

// see implementation in engine/execution/storehouse/in_memory_register_store.go
type InMemoryRegisterStore interface {
	Prune(finalizedHeight uint64, finalizedBlockID flow.Identifier) error
	PrunedHeight() uint64

	GetRegister(height uint64, blockID flow.Identifier, register flow.RegisterID) (flow.RegisterValue, error)
	GetUpdatedRegisters(height uint64, blockID flow.Identifier) ([]flow.RegisterEntry, error)
	SaveRegisters(
		height uint64,
		blockID flow.Identifier,
		parentID flow.Identifier,
		registers []flow.RegisterEntry,
	) error

	IsBlockExecuted(height uint64, blockID flow.Identifier) (bool, error)
}

type OnDiskRegisterStore = storage.RegisterIndex

type ExecutedFinalizedWAL interface {
	Append(height uint64, registers []flow.RegisterEntry) error

	// GetLatest returns the latest height in the WAL.
	Latest() (uint64, error)

	GetReader(height uint64) WALReader
}

type WALReader interface {
	// Next returns the next height and trie updates in the WAL.
	// It returns EOF when there are no more entries.
	Next() (height uint64, registers []flow.RegisterEntry, err error)
}
