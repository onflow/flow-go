package consensus

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/component"
)

// Compliance defines the interface to the consensus logic that precedes hotstuff logic.
// It's responsible for processing incoming block proposals broadcast by other consensus nodes
// as well as blocks obtained via the sync protocol.
// Compliance logic performs validation of incoming blocks depending on internal implementation.
// Main consensus logic performs full validation by checking headers and payloads.
// Follower consensus logic checks header validity and by observing a valid QC can make a statement about
// payload validity of parent block.
// Compliance logic guarantees that only valid blocks are added to chain state, passed to hotstuff and other
// components.
// Implementation need to be non-blocking and concurrency safe.
type Compliance interface {
	component.Component

	// OnBlockProposal feeds a new block proposal into the processing pipeline.
	// Incoming proposals will be queued and eventually dispatched by worker.
	// This method is non-blocking.
	OnBlockProposal(proposal flow.Slashable[*messages.BlockProposal])

	// OnSyncedBlocks feeds a batch of blocks obtained from sync into the processing pipeline.
	// Implementors shouldn't assume that blocks are arranged in any particular order.
	// Incoming proposals will be queued and eventually dispatched by worker.
	// This method is non-blocking.
	OnSyncedBlocks(blocks flow.Slashable[[]*messages.BlockProposal])
}
