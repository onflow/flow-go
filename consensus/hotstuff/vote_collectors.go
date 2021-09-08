package hotstuff

// VoteCollectors is an interface which allows VoteAggregator to interact with collectors structured by
// view and blockID.
// Implementations of this interface are responsible for state transitions of `VoteCollector`s and pruning of
// stale and outdated collectors by view.
type VoteCollectors interface {
	// GetOrCreateCollector retrieves the hotstuff.VoteCollector for the specified
	// view or creates one if none exists.
	// When creating a vote collector, the view will be used to get epoch by view, then create the random beacon
	// signer object by epoch, because epoch determines DKG, which determines random beacon committee.
	// It returns:
	//  -  (collector, true, nil) if no collector can be found by the block ID, and a new collector was created.
	//  -  (collector, false, nil) if the collector can be found by the block ID
	//  -  (nil, false, error) if running into any exception creating the vote collector state machine
	// Expected error returns during normal operations:
	//  * mempool.DecreasingPruningHeightError
	GetOrCreateCollector(view uint64) (collector VoteCollector, created bool, err error)

	// PruneUpToView prunes the vote collectors whose view is below the given view.
	// If `view` is smaller than the previous value, the previous value is kept
	// and the sentinel mempool.DecreasingPruningHeightError is returned.
	PruneUpToView(view uint64) error
}
