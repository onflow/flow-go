package state

import (
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

type StoreHouseReader interface {
	// BlockView returns a blockView allowing to query register values for specific block
	BlockView(height uint64, blockID flow.Identifier) (snapshot.StorageSnapshot, error)
}

// StoreHouse is a storage for storing register values for each block.
// Internally it will decide storing it into fork-aware storage or non-forkaware storage
// depending on whether the block has been finalized.
// The reason the two storages can not be merged is because only non-forkaware storage can
// index registers by finalized height, whereas forkaware storage has to keep track of updates
// for different forks.
type StoreHouse interface {
	StoreHouseReader

	// StoreBlock stores the updated register values for the given block into storage.
	// (it expects nil value for the removed registers in the "update" argument)
	// If the block has not been finalized, the updated registers will be stored in a fork-aware
	// in-memory storage.
	// If the block has been finalized, the updated registers will be stored in a non-forkaware
	// disk storage
	StoreBlock(header *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error

	BlockFinalized(finalized *flow.Header) error
}

// ForkAwareStorage is an fork aware in memory storage for
type ForkAwareStorage interface {
	// BlockView returns a blockView allowing to query register values for specific block
	BlockView(height uint64, blockID flow.Identifier) (snapshot.StorageSnapshot, error)

	AddForBlock(header *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error

	GetUpdatesByBlock(blockID flow.Identifier) (map[flow.RegisterID]flow.RegisterValue, error)

	// if a branch is pruned, then block execution for the pruned block might run into error
	// when getting registers from a pruned block view
	PruneByFinalized(finalized *flow.Header) error
}

type NonForkAwareStorage interface {
	// BlockView returns a blockView allowing to query register values for specific block
	BlockView(height uint64) (snapshot.StorageSnapshot, error)

	// CommitBlock stores the updated registers to disk and index them by the given height.
	// The given block must be finalized.
	CommitBlock(finalized *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error
}
