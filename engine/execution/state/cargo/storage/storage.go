package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// Storage is a block height-based persistant storage for storing historic register values.
// it accepts updates as a set of sequential fork-free commit calls and
// let user to query for the value of a register for at a given height
type Storage interface {
	// RegisterValueAt returns the register value at the given height
	// note that if a register is not updated at specific block it pretaining the previous value
	// non-existing registers return nil value by default and errors are only kept for fatal ones
	RegisterValueAt(height uint64, blockID flow.Identifier, id flow.RegisterID) (value flow.RegisterValue, err error)

	// Commit updates register values (it expects nil value for the removed registers)
	// verifies that the new update's block header is compliant (block height and parent id matches)
	CommitBlock(header *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error

	// LastCommittedBlock returns the block header for the last committed changes
	LastCommittedBlock() (*flow.Header, error)
}
