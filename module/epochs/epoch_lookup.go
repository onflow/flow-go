package epochs

import (
	"errors"
	"fmt"
	"sync"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/state/protocol"
)

// EpochLookup implements the module.EpochLookup interface using protocol state to
// match views to epochs.
type EpochLookup struct {
	state                    protocol.State
	mu                       sync.RWMutex
	epochs                   map[uint64]epochInfo
	isEpochFallbackTriggered *atomic.Bool
}

type epochInfo struct {
	firstView uint64
	finalView uint64
	counter   uint64
}

// NewEpochLookup instantiates a new EpochLookup
func NewEpochLookup(state protocol.State) *EpochLookup {
	return &EpochLookup{
		state: state,
	}
}

// EpochForView returns the counter of the epoch that the view belongs to.
// If epoch emergency fallback has been triggered, returns the current epoch
// counter for all views past the end of the current epoch
//
// Error returns:
// * model.ErrViewForUnknownEpoch if the view belongs to an epoch outside the
//     current, previous, and next epoch, based on the current finalized state.
// * any other error is a symptom of an internal error or bug
// TODO should return model.ErrViewForUnknownEpoch, move to non-Hotstuff dir
func (l *EpochLookup) EpochForView(view uint64) (epochCounter uint64, err error) {

	final := l.state.Final()
	epochs := final.Epochs()

	// common case: current epoch
	currentEpoch, ok, err := l.getEpochInfo(epochs.Current())
	if err != nil {
		return 0, fmt.Errorf("could not get current epoch info: %w", err)
	}
	if !ok {
		return 0, fmt.Errorf("failed sanity check: unable to get current current epoch")
	}
	if currentEpoch.firstView <= view && view <= currentEpoch.finalView {
		return currentEpoch.counter, nil
	}

	// check for a previous epoch
	if view < currentEpoch.firstView {
		previousEpoch, ok, err := l.getEpochInfo(epochs.Previous())
		if err != nil {
			return 0, fmt.Errorf("could not get previous epoch info: %w", err)
		}
		if ok {
			if view < previousEpoch.firstView {
				return 0, model.ErrViewForUnknownEpoch
			}
			return previousEpoch.counter, nil
		}
		return 0, model.ErrViewForUnknownEpoch
	}

	// at this point, we know that the view is after the current epoch

	// if epoch fallback is triggered, all views beyond the end of the current
	// epoch should be considered part of the current epoch
	epochFallbackTriggered, err := l.state.Params().EpochFallbackTriggered()
	if err != nil {
		return 0, fmt.Errorf("could not check if epoch fallback triggered: %w", err)
	}
	if epochFallbackTriggered {
		return currentEpoch.counter, nil
	}

	// check for a next epoch
	nextEpoch, ok, err := l.getEpochInfo(epochs.Next())
	if err != nil {
		return 0, fmt.Errorf("could not get next epoch info: %w", err)
	}
	if ok {
		if view > nextEpoch.finalView {
			return 0, model.ErrViewForUnknownEpoch
		}
		return nextEpoch.counter, nil
	}
	return 0, model.ErrViewForUnknownEpoch
}

// getEpochInfo returns the epoch counter and first/final views.
// No errors are expected during normal operation.
// Returns:
// * counter, true, nil - if the view is within the bounds of the epoch
// * counter, false, nil - if the view is not within the bounds of the epoch
// * 0, false, error - for any unexpected error conditions
func (l *EpochLookup) getEpochInfo(epoch protocol.Epoch) (*epochInfo, bool, error) {
	counter, err := epoch.Counter()
	if errors.Is(err, protocol.ErrNoPreviousEpoch) || errors.Is(err, protocol.ErrNextEpochNotSetup) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("could not get epoch counter: %w", err)
	}

	firstView, err := epoch.FirstView()
	if err != nil {
		return nil, false, fmt.Errorf("could not get first view: %w", err)
	}
	finalView, err := epoch.FinalView()
	if err != nil {
		return nil, false, fmt.Errorf("could not get final view: %w", err)
	}

	return &epochInfo{
		firstView: firstView,
		finalView: finalView,
		counter:   counter,
	}, true, nil
}
