package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// VoteCollectors is an interface which allows VoteAggregator to interact with collectors structured by
// view and blockID.
// Implementations of this interface are responsible for state transitions of `VoteCollector`s and pruning of
// stale and outdated collectors by view.
type VoteCollectors interface {
	// GetOrCreateCollector is used for getting hotstuff.VoteCollector.
	// if there is no collector created or the given block ID, then it will create one.
	// Collector is indexed by blockID for looking up by blockID, and also indexed by view for pruning.
	// It returns:
	//  -  (collector, true, nil) if no collector can be found by the block ID, and a new collector was created.
	//  -  (collector, false, nil) if the collector can be found by the block ID
	//  -  (nil, false, error) if running into any exception creating the vote collector state machine
	// TODO: do we need to verify the view? if an invalid vote has an invalid view, would we create a vote collector
	// indexed by a wrong view?
	GetOrCreateCollector(view uint64, blockID flow.Identifier) (collector VoteCollector, created bool, err error)

	// Prune the vote collectors whose view is below the given view
	PruneUpToView(view uint64) error

	// ProcessBlock performs validation of block signature and processes block with respected collector.
	// Calling this function will mark conflicting collectors as stale and change state of valid collectors
	// It returns nil if the block is valid.
	// It returns model.InvalidBlockError if block is invalid.
	// It returns other error if there is exception processing the block.
	ProcessBlock(block *model.Proposal) error
}
