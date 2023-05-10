package state

import (
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

// StoreHouse is a storage for storing register values for each block.
// Registers are updated at each block, in order to query the latest value for a given
// register, we will need to traverse along the chain of blocks until finding a block that
// has the register value updated.
// The chain of blocks might or might not have forks depending on whether the blocks are finalized.
// If a block is finalized, there is no fork below its view, therefore we can index
// the register updates by finalized view which is more effective for lookup.
// For blocks above the last finalized view, there might be multiple forks, so we need to
// store the registers updates differently.
// Therefore, we create two different storages internally (fork-aware store and non-forkware storage),
// and storing the register updates of a block depending on whether the executed block has
// been finalized or not.
// Non-forkaware storage stores register updates for finalized blocks, whereas forkaware store
// stores for non-finalized blocks.
type StoreHouse interface {
	StoreHouseReader

	// StoreBlock stores the updated register values for the given block into storage.
	// (it expects nil value for the removed registers in the "update" argument)
	// If the block has not been finalized, the updated registers will be stored in a fork-aware
	// in-memory storage.
	// If the block has been finalized, the updated registers will be stored in a non-forkaware
	// disk storage
	StoreBlock(header *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error

	// BlockFinalized notify the StoreHouse about block finalization, so that StoreHouse can
	// commit the register updates for the block and move them into non-forkaware storage
	BlockFinalized(finalized *flow.Header) error
}

type StoreHouseReader interface {
	// BlockView returns a blockView allowing to query register values for specific block
	BlockView(view uint64, blockID flow.Identifier) (snapshot.StorageSnapshot, error)
}

// ForkAwareStore is an fork aware in memory store for storing the register updates for
// unfinalized blocks.
// Once one fork is finalized, then register updates for blocks on the finalized fork
// can be moved to NonForkAwareStorage, and other conflicting forks can be pruned.
type ForkAwareStore interface {
	// GetRegsiter returns the latest register value for a given block
	// it will traverses along the fork of blocks until finding the latest updated value.
	// for instance, there are two forks of blocks as below:
	// A <- B <- C <- D
	//        ^- E <- F
	// A (key1: 2), <- this means block A updated register key1 with value 2
	// B (key2: 3)
	// C (key1: 4)
	// D (key2: 5)
	// E (key2: 6)
	// F (key2: 7)
	// GetRegsiter(10, D, key1) will return 4, because C has the latest value of key1 on
	// the fork (A<-B<-C<-D)
	// GetRegsiter(10, E, key1) will return 2, because A has the latest value of key1 on
	// the fork (A<-B<-E<-F)
	// GetRegsiter(10, F, key3) will return NotFound, because the register value is not updated
	// on the fork (A<-B<-E<-F). When register value is not found, the call needs to query it from
	// NonForkAwareStorage
	// TODO(leo): check NotFound error type
	GetRegsiter(view uint64, blockID flow.Identifier, id flow.RegisterID) (flow.RegisterValue, error)

	// AddForBlock add the register updates of a given block to ForkAwareStore
	AddForBlock(header *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error

	// GetUpdatesByBlock returns all the register updates updated at the given block.
	// Useful for moving them into NonForkAwareStorage when the given block becomes finalized.
	GetUpdatesByBlock(blockID flow.Identifier) (map[flow.RegisterID]flow.RegisterValue, error)

	// PruneByFinalized remove the register updates for all the blocks below the finalized height
	// as well as the blocks that are conflicting with the finalized block
	// Make sure the register updates for finalized blocks have been moved to non-forkaware
	// storage before pruning them from forkaware store.
	// For instance, given the following state in the forkaware store:
	// A <- B <- C <- D
	//        ^- E <- F
	// if C is finalized, then PruneByFinalized(C) will prune [A, B, E, F],
	// [A,B] are pruned because they are below finalized view,
	// [E,F] are pruned because they are conflicting with finalized block C.
	PruneByFinalized(finalized *flow.Header) error
}

// NonForkAwareStorage stores register updates for finalized blocks into database
type NonForkAwareStorage interface {
	// GetRegsiter returns the latest register value for a given block
	GetRegsiter(view uint64, blockID flow.Identifier, id flow.RegisterID) (flow.RegisterValue, error)

	// CommitBlock stores the updated registers to disk and index them by the finalized view.
	// The given block header must be finalized.
	CommitBlock(finalized *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error
}
