// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package protocol

import (
	"github.com/onflow/flow-go/model/flow"
)

// Mutator represents an interface to modify the persistent protocol state in a
// way that conserves its integrity. It enforces a number of invariants on the
// input data that ensures the internal bookkeeping mechanisms remain functional
// and valid.
type Mutator interface {

	// Bootstrap initializes the persistent protocol state with the given block,
	// execution state and block seal. In order to successfully bootstrap, the
	// execution result needs to refer to the provided block and the block seal
	// needs to refer to the provided block and execution result. The identities
	// in the block payload will be used as the initial set of staked node
	// identities.
	Bootstrap(root *flow.Block, result *flow.ExecutionResult, seal *flow.Seal) error

	// Extend introduces the block with the given ID into the persistent
	// protocol state without modifying the current finalized state. It allows
	// us to execute fork-aware queries against ambiguous protocol state, while
	// still checking that the given block is a valid extension of the protocol
	// state.
	Extend(block *flow.Block) error

	// HeaderExtend is a lighter version of Extend that checks whether the given
	// block is a valid extension of the protocol state by checking only the block
	// header.
	// Useful for consensus follower to check the block with lower cost, and
	// rely on consensus participants for the full block check, including the block
	// payload
	HeaderExtend(block *flow.Block) error

	// Finalize finalizes the block with the given hash.
	// At this level, we can only finalize one block at a time. This implies
	// that the parent of the pending block that is to be finalized has
	// to be the last finalized block.
	// It modifies the persistent immutable protocol state accordingly and
	// forwards the pointer to the latest finalized state.
	Finalize(blockID flow.Identifier) error

	// MarkValid marks the block header with the given block hash as valid.
	// At this level, we can only mark one block at a time as valid. This
	// implies that the parent of the block to be marked as valid
	// has to be already valid.
	// It modifies the persistent immutable protocol state accordingly.
	MarkValid(blockID flow.Identifier) error
}

type MutatorFactory interface {
	Create(state *State) Mutator
}
