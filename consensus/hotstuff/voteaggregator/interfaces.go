package voteaggregator

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// VoteCollectors is an interface which allows VoteAggregator to interact with collectors structured by
// view and blockID.
// Implementations of this interface are responsible for state transitions of `VoteCollector`s and pruning of
// stale and outdated collectors by view.
type VoteCollectors interface {
	// GetOrCreateCollector is used for getting hotstuff.VoteCollector, calling this function for first time
	// will create a new collector.
	// collector is indexed by blockID and view
	// It returns the vote collector state machine and true if found,
	// It returns nil and false if not found
	GetOrCreateCollector(view uint64, blockID flow.Identifier) (hotstuff.VoteCollector, bool)

	// Prune the vote collectors whose view is below the given view
	PruneUpToView(view uint64) error

	// ProcessBlock performs validation of block signature and processes block with respected collector.
	// Calling this function will mark conflicting collectors as stale and change state of valid collectors
	// It returns nil if the block is valid.
	// It returns model.InvalidBlockError if block is invalid.
	// It returns other error if there is exception processing the block.
	ProcessBlock(block *model.Block) error
}
