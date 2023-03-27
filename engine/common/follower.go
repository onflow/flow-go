package common

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// FollowerCore interface defines the methods that a consensus follower must implement in order to synchronize
// with the Flow network.
// FollowerCore processes incoming continuous ranges of blocks by executing consensus follower logic which consists of
// validation and extending protocol state by applying state changes contained in block's payload.
// Processing valid ranges of blocks results in extending protocol state and subsequent finalization of pending blocks.
type FollowerCore interface {
	module.Startable
	module.ReadyDoneAware
	// OnBlockRange is called when a batch of blocks is received from the network.
	// The originID parameter identifies the node that sent the batch of blocks.
	// The connectedRange parameter contains the blocks, they must form a sequence of connected blocks.
	// No errors are expected during normal operations.
	// Implementors need to ensure that this function is safe to be used in concurrent environment.
	OnBlockRange(originID flow.Identifier, connectedRange []*flow.Block) error
	// OnFinalizedBlock is called when a new block is finalized by Hotstuff.
	// FollowerCore updates can update its local state using this information.
	// Implementors need to ensure that this function is safe to be used in concurrent environment.
	OnFinalizedBlock(finalized *flow.Header)
}
