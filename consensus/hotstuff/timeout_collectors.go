package hotstuff

// TimeoutCollectors encapsulates the functionality to generate, store and prune `TimeoutCollector`
// instances (one per view). Its main purpose is to provide a higher-level API to `TimeoutAggregator`
// for managing and interacting with the view-specific `TimeoutCollector` instances.
// Implementations are concurrency safe.
type TimeoutCollectors interface {
	// GetOrCreateCollector retrieves the TimeoutCollector for the specified
	// view or creates one if none exists.  When creating a timeout collector,
	// the view is used to query the consensus committee for the respective
	// Epoch the view belongs to.
	// It returns:
	//  -  (collector, true, nil) if no collector can be found by the view, and a new collector was created.
	//  -  (collector, false, nil) if the collector can be found by the view.
	//  -  (nil, false, error) if running into any exception creating the timeout collector.
	// Expected error returns during normal operations:
	//  * mempool.BelowPrunedThresholdError if view is below the pruning threshold
	//  * model.ErrViewForUnknownEpoch if view is not yet pruned but no epoch containing the given view is known
	GetOrCreateCollector(view uint64) (collector TimeoutCollector, created bool, err error)

	// PruneUpToView prunes the timeout collectors with views _below_ the given value, i.e.
	// we only retain and process timeout collectors, whose views are equal or larger than `lowestRetainedView`.
	// If `lowestRetainedView` is smaller than the previous value, the previous value is
	// kept and the method call is a NoOp.
	PruneUpToView(lowestRetainedView uint64)
}
