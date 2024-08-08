package epochs

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
)

const (
	invalidExtensionFinalView = "sanity check failed: latest epoch final view %d greater than extension final view %d"
	mismatchEpochCounter      = "sanity check failed: latest epoch counter %d does not match extension epoch counter %d"
	invalidEpochViewSequence  = "sanity check: first view of the epoch extension %d should immediately start after the final view of the latest epoch %d"
)

// epochRange captures the counter and view range of an epoch (inclusive on both ends)
type epochRange struct {
	counter   uint64
	firstView uint64
	finalView uint64
}

// exists returns true when the epochRange is initialized (anything besides the zero value for the struct).
// It is useful for checking existence while iterating the epochRangeCache.
func (er epochRange) exists() bool {
	return er != epochRange{}
}

// epochRangeCache stores at most the 3 latest epoch ranges.
// Ranges are ordered by counter (ascending) and right-aligned.
// For example, if we only have one epoch cached, `epochRangeCache[0]` and `epochRangeCache[1]` are `nil`.
// Not safe for concurrent use.
type epochRangeCache [3]epochRange

// latest returns the latest cached epoch range, or nil if no epochs are cached.
func (cache *epochRangeCache) latest() epochRange {
	return cache[2]
}

// extendLatestEpoch updates the final view of the latest epoch with the final view of the epoch extension.
// No errors are expected during normal operation.
func (cache *epochRangeCache) extendLatestEpoch(epochCounter uint64, extension flow.EpochExtension) error {
	latestEpoch := cache[2]
	// sanity check: latest epoch should already be cached.
	if !latestEpoch.exists() {
		return fmt.Errorf("sanity check failed: latest epoch does not exist")
	}

	// duplicate events are no-ops
	if latestEpoch.finalView == extension.FinalView {
		return nil
	}

	// sanity check: `extension.FinalView` should be greater than final view of latest epoch
	if cache[2].finalView > extension.FinalView {
		return fmt.Errorf(invalidExtensionFinalView, cache[2].finalView, extension.FinalView)
	}

	// sanity check: epoch extension should have the same epoch counter as the latest epoch
	if cache[2].counter != epochCounter {
		return fmt.Errorf(mismatchEpochCounter, cache[2].counter, epochCounter)
	}

	// sanity check: first view of the epoch extension should immediately start after the final view of the latest epoch.
	if latestEpoch.finalView+1 != extension.FirstView {
		return fmt.Errorf(invalidEpochViewSequence, extension.FirstView, latestEpoch.finalView)
	}

	cache[2].finalView = extension.FinalView
	return nil
}

// combinedRange returns the endpoints of the combined view range of all cached
// epochs. In particular, we return the lowest firstView and the greatest finalView.
// At least one epoch must already be cached, otherwise this function will panic.
func (cache *epochRangeCache) combinedRange() (firstView uint64, finalView uint64) {

	// low end of the range is the first view of the first cached epoch
	for _, epoch := range cache {
		if epoch.exists() {
			firstView = epoch.firstView
			break
		}
	}
	// high end of the range is the final view of the latest cached epoch
	finalView = cache.latest().finalView
	return
}

// add inserts an epoch range to the cache.
// Validates that epoch counters and view ranges are sequential.
// Adding the same epoch multiple times is a no-op.
// Guarantees ordering and alignment properties of epochRangeCache are preserved.
// No errors are expected during normal operation.
func (cache *epochRangeCache) add(epoch epochRange) error {

	// sanity check: ensure the epoch we are adding is considered a non-zero value
	// this helps ensure internal consistency in this component, but if we ever trip this check, something is seriously wrong elsewhere
	if !epoch.exists() {
		return fmt.Errorf("sanity check failed: caller attempted to cache invalid zero epoch")
	}

	latestCachedEpoch := cache.latest()
	// initial case - no epoch ranges are stored yet
	if !latestCachedEpoch.exists() {
		cache[2] = epoch
		return nil
	}

	// adding the same epoch multiple times is a no-op
	if latestCachedEpoch == epoch {
		return nil
	}

	// sanity check: ensure counters/views are sequential
	if epoch.counter != latestCachedEpoch.counter+1 {
		return fmt.Errorf("non-sequential epoch counters: adding epoch %d when latest cached epoch is %d", epoch.counter, latestCachedEpoch.counter)
	}
	if epoch.firstView != latestCachedEpoch.finalView+1 {
		return fmt.Errorf("non-sequential epoch view ranges: adding range [%d,%d] when latest cached range is [%d,%d]",
			epoch.firstView, epoch.finalView, latestCachedEpoch.firstView, latestCachedEpoch.finalView)
	}

	// typical case - displacing existing epoch ranges
	// insert new epoch range, shifting existing epochs left
	cache[0] = cache[1] // ejects oldest epoch
	cache[1] = cache[2]
	cache[2] = epoch

	return nil
}

// EpochLookup implements the EpochLookup interface using protocol state to match views to epochs.
// CAUTION: EpochLookup should only be used for querying the previous, current, or next epoch.
type EpochLookup struct {
	state  protocol.State
	mu     sync.RWMutex
	epochs epochRangeCache
	// epochEvents queues functors for processing epoch-related protocol events.
	// Events will be processed in the order they are received (fifo).
	epochEvents chan func() error
	events.Noop // implements protocol.Consumer
	component.Component
}

var _ protocol.Consumer = (*EpochLookup)(nil)
var _ module.EpochLookup = (*EpochLookup)(nil)

// NewEpochLookup instantiates a new EpochLookup
func NewEpochLookup(state protocol.State) (*EpochLookup, error) {
	lookup := &EpochLookup{
		state:       state,
		epochEvents: make(chan func() error, 20),
	}

	lookup.Component = component.NewComponentManagerBuilder().
		AddWorker(lookup.handleProtocolEvents).
		Build()

	final := state.Final()

	// we cache the previous epoch, if one exists
	exists, err := protocol.PreviousEpochExists(final)
	if err != nil {
		return nil, fmt.Errorf("could not check previous epoch exists: %w", err)
	}
	if exists {
		err := lookup.cacheEpoch(final.Epochs().Previous())
		if err != nil {
			return nil, fmt.Errorf("could not prepare previous epoch: %w", err)
		}
	}

	// we always cache the current epoch
	err = lookup.cacheEpoch(final.Epochs().Current())
	if err != nil {
		return nil, fmt.Errorf("could not prepare current epoch: %w", err)
	}

	// we cache the next epoch, if it is committed
	phase, err := final.EpochPhase()
	if err != nil {
		return nil, fmt.Errorf("could not check epoch phase: %w", err)
	}
	if phase == flow.EpochPhaseCommitted {
		err := lookup.cacheEpoch(final.Epochs().Next())
		if err != nil {
			return nil, fmt.Errorf("could not prepare previous epoch: %w", err)
		}
	}

	return lookup, nil
}

// cacheEpoch caches the given epoch's view range. Must only be called with committed epochs.
// No errors are expected during normal operation.
func (lookup *EpochLookup) cacheEpoch(epoch protocol.Epoch) error {
	counter, err := epoch.Counter()
	if err != nil {
		return err
	}
	firstView, err := epoch.FirstView()
	if err != nil {
		return err
	}
	finalView, err := epoch.FinalView()
	if err != nil {
		return err
	}

	cachedEpoch := epochRange{
		counter:   counter,
		firstView: firstView,
		finalView: finalView,
	}

	lookup.mu.Lock()
	err = lookup.epochs.add(cachedEpoch)
	lookup.mu.Unlock()
	if err != nil {
		return fmt.Errorf("could not add epoch %d: %w", counter, err)
	}
	return nil
}

// EpochForView returns the counter of the epoch that the input view belongs to.
// Note: The EpochLookup component processes EpochExtended notifications which will
// extend the view range for the latest epoch.
//
// Returns model.ErrViewForUnknownEpoch if the input does not fall within the range of a known epoch.
func (lookup *EpochLookup) EpochForView(view uint64) (uint64, error) {
	lookup.mu.RLock()
	defer lookup.mu.RUnlock()
	firstView, finalView := lookup.epochs.combinedRange()

	// LEGEND:
	// *      -> view argument
	// [----| -> epoch view range

	// view is before any known epochs
	// ---*---[----|----|----]-------
	if view < firstView {
		return 0, model.ErrViewForUnknownEpoch
	}
	// view is after any known epochs
	// -------[----|----|----]---*---
	if view > finalView {
		// otherwise, we are waiting for the epoch including this view to be committed
		return 0, model.ErrViewForUnknownEpoch
	}

	// view is within a known epoch
	for _, epoch := range lookup.epochs {
		if !epoch.exists() {
			continue
		}
		if epoch.firstView <= view && view <= epoch.finalView {
			return epoch.counter, nil
		}
	}

	// reaching this point indicates a corrupted state or internal bug
	return 0, fmt.Errorf("sanity check failed: cached epochs (%v) does not contain input view %d", lookup.epochs, view)
}

// handleProtocolEvents processes queued Epoch events `EpochCommittedPhaseStarted`
// and `EpochExtended`. This function permanently utilizes a worker
// routine until the `Component` terminates.
// When we observe a new epoch being committed, we compute
// the leader selection and cache static info for the epoch.
func (lookup *EpochLookup) handleProtocolEvents(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case processEventFn := <-lookup.epochEvents:
			err := processEventFn()
			if err != nil {
				ctx.Throw(err)
			}
		}
	}
}

// EpochExtended listens to `EpochExtended` protocol notifications which the Protocol
// State emits when we finalize the first block whose Protocol State further extends the current
// epoch. The next epoch should not be committed so far, because epoch extension are only added
// when there is no subsequent epoch that we could transition into but the current epoch is nearing
// its end. The notification is queued for async processing by the worker.
// Specifically, we update the final view of the latest epoch range with the final view of the
// current epoch, which will now be updated because the epoch has extensions.
// We must process _all_ `EpochExtended` notifications.
// No errors are expected to be returned by the process callback during normal operation.
func (lookup *EpochLookup) EpochExtended(epochCounter uint64, _ *flow.Header, extension flow.EpochExtension) {
	lookup.epochEvents <- func() error {
		err := lookup.epochs.extendLatestEpoch(epochCounter, extension)
		if err != nil {
			return err
		}

		return nil
	}
}

// EpochCommittedPhaseStarted ingests the respective protocol notifications
// which the Protocol State emits when we finalize the first block whose Protocol State further extends the current
// epoch. The notification is queued for async processing by the worker. Specifically, we cache the next epoch in the EpochLookup.
// We must process _all_ `EpochCommittedPhaseStarted` notifications.
// No errors are expected to be returned by the process callback during normal operation.
func (lookup *EpochLookup) EpochCommittedPhaseStarted(_ uint64, first *flow.Header) {
	lookup.epochEvents <- func() error {
		epoch := lookup.state.AtBlockID(first.ID()).Epochs().Next()
		err := lookup.cacheEpoch(epoch)
		if err != nil {
			return fmt.Errorf("failed to cache next epoch: %w", err)
		}

		return nil
	}
}
