// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package protocol

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Mutator represents an interface to modify the persistent protocol state in a
// way that conserves its integrity. It enforces a number of invariants on the
// input data that ensures the internal bookkeeping mechanisms remain functional
// and valid.
type Mutator interface {

	// Bootstrap initializes the persistent protocol state with the given block,
	// execution state and block seal. In order to successfully bootstrap, the
	// execution result needs to refer to the provided block and the block seal
	// needs to refer to the provided block and eecution result. The identities
	// in the block payload will be used as the initial set of staked node
	// identities.
	Bootstrap(block *flow.Block, result *flow.ExecutionResult, seal *flow.Seal) error

	// Extend introduces the block with the given ID into the persistent
	// protocol state without modifying the current finalized state. It allows
	// us to execute fork-aware queries against ambiguous protocol state, while
	// still checking that the given block is a valid extension of the protocol
	// state.
	Extend(block *flow.Block) error

	// Finalize finalizes the block with the given hash, and all of its parents
	// up to the finalized protocol state. It modifies the persistent immutable
	// protocol state accordingly and forwards the pointer to the latest
	// finalized state.
	Finalize(blockID flow.Identifier) error
}
