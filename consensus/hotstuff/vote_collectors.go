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
	// Collector is indexed by (blockID + view) for looking up collector by (blockID + view),
	// and also indexed by view for pruning.
	// Why indexing by blockID and view instead of blockID only?
	// Because if a vote has incorrect view would cause us creating a vote collector indexed by
	// a wrong view, and this vote collector might got pruned by the wrong view while it has
	// collected votes from the valid block and other valid votes.
	// Indexing by both blockID and view allows us to index vote collectors different for valid and
	// invalid votes.
	// When creating a vote collector, the view will be used to get epoch by view, then create the random beacon
	// signer object by epoch, because epoch determines DKG, which determines random beacon committee.
	// It returns:
	//  -  (collector, true, nil) if no collector can be found by the block ID, and a new collector was created.
	//  -  (collector, false, nil) if the collector can be found by the block ID
	//  -  (nil, false, error) if running into any exception creating the vote collector state machine
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
