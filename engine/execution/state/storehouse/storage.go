package storehouse

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

// Storage is a block height-based persistant storage for storing historic register values.
// it accepts updates as a set of sequential fork-free commit calls and
// let user to query for the value of a register for at a given height
type Storage interface {
	// TODO(ramtin) maybe define a new type here holding both blockID and height
	// BlockView returns an object allowing to query register values for specific block
	BlockView(height uint64, blockID flow.Identifier) (BlockView, error)

	// Commit updates register values (it expects nil value for the removed registers)
	// verifies that the new update's block header is compliant (block height and parent id matches)
	CommitBlock(header *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error

	// LastCommittedBlock returns the block header for the last committed changes
	LastCommittedBlock() (*flow.Header, error)
}
