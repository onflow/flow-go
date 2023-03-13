package common

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

// FollowerCore abstracts event handlers for specific events that can be received from engine-level logic.
// Whenever engine receives a message which needs interaction with follower logic it needs to be passed down
// the pipeline using this interface.
type FollowerCore interface {
	// OnBlockProposal handles incoming block proposals obtained from sync engine.
	// Performs core processing logic.
	// Is NOT concurrency safe.
	// No errors are expected during normal operations.
	OnBlockProposal(originID flow.Identifier, proposal *messages.BlockProposal) error

	// OnFinalizedBlock handles new finalized block. When new finalized block has been detected this
	// function is expected to be called to inform core logic about new finalized state.
	// Is NOT concurrency safe.
	// No errors are expected during normal operations.
	OnFinalizedBlock(block *flow.Header) error
}
