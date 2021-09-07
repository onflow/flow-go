package hotstuff

import (
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
	// When creating a vote collector, the view will be used to get epoch by view, then create the random beacon
	// signer object by epoch, because epoch determines DKG, which determines random beacon committee.
	// It returns:
	//  -  (collector, true, nil) if no collector can be found by the block ID, and a new collector was created.
	//  -  (collector, false, nil) if the collector can be found by the block ID
	//  -  (nil, false, error) if running into any exception creating the vote collector state machine
	GetOrCreateCollector(view uint64, blockID flow.Identifier) (collector VoteCollector, created bool, err error)

	// Prune the vote collectors whose view is below the given view
	PruneUpToView(view uint64) error
}
