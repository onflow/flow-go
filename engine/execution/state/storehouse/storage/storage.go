package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// BlockView returns register values at specific block
type BlockView interface {
	// Register returns the register value at this block
	// note that if a register is not updated at specific block it pretaining the previous value
	// non-existing registers return nil value by default and errors are only kept for fatal ones
	Get(id flow.RegisterID) (value flow.RegisterValue, err error)
}

// Storage is a storage for storing register values for each block (non-forkaware).
// it accepts updates as a set of sequential commit calls and
// let user to query for the value of a register for at a given height (historic lookup)
type Storage interface {
	// BlockView returns a blockView allowing to query register values for specific block
	BlockView(height uint64, blockID flow.Identifier) (BlockView, error)

	// Commit updates register values (it expects nil value for the removed registers)
	// verifies that the new update's block header is compliant (block height and parent id matches)
	CommitBlock(header *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error

	// LastCommittedBlock returns the block header for the last committed changes
	LastCommittedBlock() (*flow.Header, error)
}

// ForkAwareStorage is a fork-aware version of storage
// it consumes block finalization signals to prune and update
type ForkAwareStorage interface {
	Storage

	// BlockFinalized receives block finalization calls in the order blocks getting finalized (no-concurrent call)
	// if the storage hasn't received the block execution results for this block it would return false
	BlockFinalized(header *flow.Header) (bool, error)
}
