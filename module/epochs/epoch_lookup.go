package epochs

import (
	"errors"
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

// viewRange captures the counter and view range of an epoch (inclusive on both ends)
// The zero value (epochCounter = firstView = finalView = 0) can conceptually not occur:
// even the genesis epoch '0' is required to have some views to prepare the subsequent epoch.
// Therefore, we use the zero value to represent 'undefined'.
type viewRange struct {
	epochCounter uint64
	firstView    uint64
	finalView    uint64
}

// undefined returns true when the viewRange is not initialized (equal to the zero value for the struct).
// It is useful for checking existence while iterating the epochRangeCache.
func (er viewRange) undefined() bool {
	return er.epochCounter == 0 && er.firstView == 0 && er.finalView == 0
}

// epochRangeCache stores at most the 3 latest epoch `viewRange`s.
// viewRange are ordered by counter (ascending) and right-aligned. For example, if we only have one epoch
// cached, `epochRangeCache[0]` and `epochRangeCache[1]` are `undefined`. epochRangeCache enforces the
// following conventions:
//  1. For two adjacent epochs, their view ranges must form a continuous sequence without gaps or overlap.
//     Formally: epochRangeCache[i].finalView + 1 = epochRangeCache[i+1].firstView must hold for _all_ cached epochs.
//  2. For any two adjacent epochs, their the view counters must satisfy:
//     epochRangeCache[i].epochCounter + 1 = epochRangeCache[i+1].epochCounter
//
// NOT safe for CONCURRENT use.
type epochRangeCache [3]viewRange

// latest returns the latest cached epoch range. Follows map semantics
// to indicate whether latest is known. If the boolean return value is false, the returned view range
// is the zero value, i.e. undefined.
func (cache *epochRangeCache) latest() (viewRange, bool) {
	return cache[2], !cache[2].undefined()
}

// extendLatestEpoch updates the final view of the latest epoch with the final view of the epoch extension.
// No errors are expected during normal operation.
func (cache *epochRangeCache) extendLatestEpoch(epochCounter uint64, extension flow.EpochExtension) error {
	latestEpoch := cache[2]
	// sanity check: latest epoch should already be cached.
	if latestEpoch.undefined() {
		return fmt.Errorf("sanity check failed: latest epoch does not exist")
	}

	// duplicate events are no-ops
	if latestEpoch.finalView == extension.FinalView {
		return nil
	}

	// sanity check: `extension.FinalView` should be greater than final view of latest epoch
	if latestEpoch.finalView > extension.FinalView {
		return fmt.Errorf(invalidExtensionFinalView, latestEpoch.finalView, extension.FinalView)
	}

	// sanity check: epoch extension should have the same epoch counter as the latest epoch
	if latestEpoch.epochCounter != epochCounter {
		return fmt.Errorf(mismatchEpochCounter, latestEpoch.epochCounter, epochCounter)
	}

	// sanity check: first view of the epoch extension should immediately start after the final view of the latest epoch.
	if latestEpoch.finalView+1 != extension.FirstView {
		return fmt.Errorf(invalidEpochViewSequence, extension.FirstView, latestEpoch.finalView)
	}

	cache[2].finalView = extension.FinalView
	return nil
}

// cachedEpochs returns a slice of the cached epochs in order. The return slice is guaranteed to satisfy:
//  1. For two adjacent epochs, their view ranges form a continuous sequence without gaps or overlap.
//     Formally: epochRangeCache[i].finalView + 1 = epochRangeCache[i+1].firstView
//  2. For any two adjacent epochs, their epoch counters increment by one
//     epochRangeCache[i].epochCounter + 1 = epochRangeCache[i+1].epochCounter
//  3. All slice elements are different from the zero value (i.e. not undefined).
//
// If no elements are cached, the return slice is empty/nil. It may also contain only a single epoch.
func (cache *epochRangeCache) cachedEpochs() []viewRange {
	for i, epoch := range cache {
		if epoch.undefined() {
			continue
		}
		return cache[i:3]
	}
	return nil
}

// add inserts an epoch range to the cache.
// Validates that epoch counters and view ranges are sequential.
// Adding the same epoch multiple times is a no-op.
// Guarantees ordering and alignment properties of epochRangeCache are preserved.
// No errors are expected during normal operation.
func (cache *epochRangeCache) add(epoch viewRange) error {
	// sanity check: ensure the epoch we are adding is non-zero value. This helps ensure internal consistency
	// in this component, but if we ever trip this check, something is seriously wrong elsewhere!
	if epoch.undefined() {
		return fmt.Errorf("sanity check failed: caller attempted to cache invalid zero epoch")
	}

	latestCachedEpoch, exists := cache.latest()
	if !exists { // initial case - no epoch ranges are stored yet
		cache[2] = epoch
		return nil
	}

	// adding the same epoch multiple times is a no-op
	if latestCachedEpoch == epoch {
		return nil
	}

	// sanity check: ensure counters/views are sequential
	if epoch.epochCounter != latestCachedEpoch.epochCounter+1 {
		return fmt.Errorf("non-sequential epoch counters: adding epoch %d when latest cached epoch is %d", epoch.epochCounter, latestCachedEpoch.epochCounter)
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
// Only Epochs that are fully committed can be retrieved.
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

	epochs := state.Final().Epochs()
	prev, err := epochs.Previous()
	if err != nil {
		if !errors.Is(err, protocol.ErrNoPreviousEpoch) {
			return nil, irrecoverable.NewExceptionf("unexpected error while retrieving previous epoch: %w", err)
		}
		// `ErrNoPreviousEpoch` is an expected edge case during normal operations (e.g. we are in first epoch after spork)
		// continue without caching the previous epoch
	} else { // previous epoch was successfully retrieved
		err = lookup.cacheEpoch(prev)
		if err != nil {
			return nil, fmt.Errorf("could not prepare previous epoch: %w", err)
		}
	}

	// we always cache the current epoch
	curr, err := epochs.Current()
	if err != nil {
		return nil, fmt.Errorf("could not get current epoch: %w", err)
	}
	err = lookup.cacheEpoch(curr)
	if err != nil {
		return nil, fmt.Errorf("could not prepare current epoch: %w", err)
	}

	// we cache the next epoch, if it is committed
	nextEpoch, err := epochs.NextCommitted()
	if err != nil {
		if !errors.Is(err, protocol.ErrNextEpochNotCommitted) {
			return nil, irrecoverable.NewExceptionf("unexpected error retrieving next epoch: %w", err)
		}
		// receiving a `ErrNextEpochNotCommitted` is expected during the happy path
	} else { // next epoch was successfully retrieved
		err = lookup.cacheEpoch(nextEpoch)
		if err != nil {
			return nil, fmt.Errorf("could not cache next committed epoch: %w", err)
		}
	}

	return lookup, nil
}

// cacheEpoch caches the given epoch's view range. Must only be called with committed epochs.
// No errors are expected during normal operation.
func (lookup *EpochLookup) cacheEpoch(epoch protocol.CommittedEpoch) error {
	counter := epoch.Counter()
	cachedEpoch := viewRange{
		epochCounter: counter,
		firstView:    epoch.FirstView(),
		finalView:    epoch.FinalView(),
	}

	lookup.mu.Lock()
	err := lookup.epochs.add(cachedEpoch)
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

	cachedEpochs := lookup.epochs.cachedEpochs()
	l := len(cachedEpochs)
	if l == 0 {
		return 0, model.ErrViewForUnknownEpoch
	}

	// by convention, `epochRangeCache` guarantees that epochs are successive, without gaps or overlaps
	// in their epoch counters and view ranges. Therefore, we can just chronologically walk through the
	// epochs. This is an internal lookup, so we optimize for the happy path and proceed reverse chronologically
	// LEGEND:
	// *      -> view argument
	// [----| -> epoch view range

	// view is after the known epochs
	// -------[----|----|----]---*---
	if view > cachedEpochs[l-1].finalView {
		// The epoch including this view may be close to being committed. However, until an epoch
		// is committed, views cannot be conclusively assigned to it.
		return 0, model.ErrViewForUnknownEpoch
	}
	// view is within a known epoch
	// -------[----|-*--|----]-------
	for i := l - 1; i >= 0; i-- {
		if cachedEpochs[i].firstView <= view && view <= cachedEpochs[i].finalView {
			return cachedEpochs[i].epochCounter, nil
		}
	}
	// view is before any known epochs
	// ---*---[----|----|----]-------
	if view < cachedEpochs[0].firstView {
		return 0, model.ErrViewForUnknownEpoch
	}

	// reaching this point indicates a corrupted state or internal bug
	return 0, fmt.Errorf("sanity check failed: input view %d falls within the view range [%d, %d] of the cached epochs epochs, but none contains it", view, cachedEpochs[0].firstView, cachedEpochs[l-1].finalView)
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
		epoch, err := lookup.state.AtBlockID(first.ID()).Epochs().NextCommitted()
		if err != nil {
			return fmt.Errorf("could not get next committed epoch: %w", err)
		}
		err = lookup.cacheEpoch(epoch)
		if err != nil {
			return fmt.Errorf("failed to cache next epoch: %w", err)
		}

		return nil
	}
}
