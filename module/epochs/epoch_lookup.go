package epochs

import (
	"fmt"
	"sync"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
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
	state                    protocol.State
	mu                       sync.RWMutex
	epochs                   epochRangeCache
	committedEpochsCh        chan *flow.Header // protocol events for newly committed epochs (the first block of the epoch is passed over the channel)
	epochFallbackIsTriggered *atomic.Bool      // true when epoch fallback is triggered
	events.Noop                                // implements protocol.Consumer
	component.Component
}

var _ protocol.Consumer = (*EpochLookup)(nil)
var _ module.EpochLookup = (*EpochLookup)(nil)

// NewEpochLookup instantiates a new EpochLookup
func NewEpochLookup(state protocol.State) (*EpochLookup, error) {
	lookup := &EpochLookup{
		state:                    state,
		committedEpochsCh:        make(chan *flow.Header, 1),
		epochFallbackIsTriggered: atomic.NewBool(false),
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
	phase, err := final.Phase()
	if err != nil {
		return nil, fmt.Errorf("could not check epoch phase: %w", err)
	}
	if phase == flow.EpochPhaseCommitted {
		err := lookup.cacheEpoch(final.Epochs().Next())
		if err != nil {
			return nil, fmt.Errorf("could not prepare previous epoch: %w", err)
		}
	}

	epochStateSnapshot, err := final.EpochProtocolState()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch protocol state: %w", err)
	}

	// if epoch fallback was triggered, note it here
	// TODO: consider replacing with phase check when it's available
	if epochStateSnapshot.EpochFallbackTriggered() {
		lookup.epochFallbackIsTriggered.Store(true)
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

// EpochForViewWithFallback returns the counter of the epoch that the input view belongs to.
// If epoch fallback has been triggered, returns the last committed epoch counter
// in perpetuity for any inputs beyond the last committed epoch view range.
// For example, if we trigger epoch fallback during epoch 10, and reach the final
// view of epoch 10 before epoch 11 has finished being setup, this function will
// return 10 even for input views beyond the final view of epoch 10.
//
// Returns model.ErrViewForUnknownEpoch if the input does not fall within the range of a known epoch.
func (lookup *EpochLookup) EpochForViewWithFallback(view uint64) (uint64, error) {
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
		// if epoch fallback is triggered, we treat this view as part of the last committed epoch
		if lookup.epochFallbackIsTriggered.Load() {
			return lookup.epochs.latest().counter, nil
		}
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
// and `EpochEmergencyFallbackTriggered`. This function permanently utilizes a worker
// routine until the `Component` terminates.
// When we observe a new epoch being committed, we compute
// the leader selection and cache static info for the epoch. When we observe
// epoch emergency fallback being triggered, we inject a fallback epoch.
func (lookup *EpochLookup) handleProtocolEvents(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case block := <-lookup.committedEpochsCh:
			epoch := lookup.state.AtBlockID(block.ID()).Epochs().Next()
			err := lookup.cacheEpoch(epoch)
			if err != nil {
				ctx.Throw(err)
			}
		}
	}
}

// EpochCommittedPhaseStarted informs the `committee.Consensus` that the block starting the Epoch Committed Phase has been finalized.
func (lookup *EpochLookup) EpochCommittedPhaseStarted(_ uint64, first *flow.Header) {
	lookup.committedEpochsCh <- first
}

// EpochEmergencyFallbackTriggered passes the protocol event to the worker thread.
func (lookup *EpochLookup) EpochEmergencyFallbackTriggered() {
	lookup.epochFallbackIsTriggered.Store(true)
}
