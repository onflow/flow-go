package hotstuff

// TimeoutCollectors is an interface which allows TimeoutCollector to interact with collectors structured by
// view.
// Implementations of this interface are responsible for state transitions of `TimeoutCollector`s and pruning of
// stale and outdated collectors by view.
type TimeoutCollectors interface {
	// GetOrCreateCollector retrieves the hotstuff.TimeoutCollector for the specified
	// view or creates one if none exists.
	// When creating a timeout collector, the view will be used to get epoch by view, then create the staking
	// signer object by epoch, because epoch determines committee.
	// It returns:
	//  -  (collector, true, nil) if no collector can be found by the view, and a new collector was created.
	//  -  (collector, false, nil) if the collector can be found by the view.
	//  -  (nil, false, error) if running into any exception creating the timeout collector.
	// Expected error returns during normal operations:
	//  * mempool.DecreasingPruningHeightError
	GetOrCreateCollector(view uint64) (collector TimeoutCollector, created bool, err error)

	// PruneUpToView prunes the timeout collectors with views _below_ the given value, i.e.
	// we only retain and process whose view is equal or larger than `lowestRetainedView`.
	// If `lowestRetainedView` is smaller than the previous value, the previous value is
	// kept and the method call is a NoOp.
	PruneUpToView(lowestRetainedView uint64)
}
