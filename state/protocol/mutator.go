// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package protocol

import (
	"github.com/onflow/flow-go/model/flow"
)

type MutableState interface {
	State
	// Extend introduces the block with the given ID into the persistent
	// protocol state without modifying the current finalized state. It allows
	// us to execute fork-aware queries against ambiguous protocol state, while
	// still checking that the given block is a valid extension of the protocol
	// state. Depending on implementation it might be a lighter version that checks only
	// block header.
	Extend(candidate *flow.Block) error

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
