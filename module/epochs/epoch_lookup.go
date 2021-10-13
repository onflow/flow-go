package epochs

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/state/protocol"
)

// EpochLookup implements the EpochLookup interface using protocol state to
// match views to epochs.
type EpochLookup struct {
	state protocol.State
}

// NewEpochLookup instantiates a new EpochLookup
func NewEpochLookup(state protocol.State) *EpochLookup {
	return &EpochLookup{
		state: state,
	}
}

// EpochForView returns the counter of the epoch that the view belongs to.
// The protocol.State#Epochs object exposes previous, current, and next epochs,
// which should be all we need. In general, we can't guarantee that a node will
// have access to epoch data beyond these three, so it is safe to throw an error
// for a query that doesn't fit within the view bounds of these three epochs
// (even if the node does happen to have that stored in the underlying storage)
// -- these queries indicate a bug in the querier.
//
func (l *EpochLookup) EpochForView(view uint64) (epochCounter uint64, err error) {
	epochs := l.state.Final().Epochs()
	previous := epochs.Previous()
	current := epochs.Current()
	next := epochs.Next()

	for _, epoch := range []protocol.Epoch{previous, current, next} {
		counter, err := epoch.Counter()
		if errors.Is(err, protocol.ErrNoPreviousEpoch) {
			continue
		}
		if err != nil {
			return 0, err
		}

		firstView, err := epoch.FirstView()
		if err != nil {
			return 0, err
		}
		finalView, err := epoch.FinalView()
		if err != nil {
			return 0, err
		}
		if firstView <= view && view <= finalView {
			return counter, nil
		}
	}

	return 0, fmt.Errorf("couldn't get epoch for view %d", view)
}

// EpochForViewWithFallback returns the counter of the epoch that the input
// view belongs to, with the same rules as EpochForView, except that this
// function will return the last committed epoch counter in perpetuity in the
// case that any epoch preparation. For example, if we are in epoch 10, and
// reach the final view of epoch 10 before epoch 11 has finished being setup,
// this function will return 10 even after the final view of epoch 10.
//
func (l *EpochLookup) EpochForViewWithFallback(view uint64) (uint64, error) {

	epochs := l.state.Final().Epochs()
	current := epochs.Current()
	next := epochs.Next()

	// TMP: EMERGENCY EPOCH CHAIN CONTINUATION [EECC]
	//
	// If the given view is within the bounds of the next epoch, and the epoch
	// has not been set up or committed, we pretend that we are still in the
	// current epoch and return that epoch's counter.
	//
	// This is used to determine which Random Beacon key we will use to sign and
	// verify blocks and votes. The failure case we are avoiding here is if the
	// DKG for next epoch failed and there is no Random Beacon key for that epoch,
	// or if the next epoch failed for any other reason. In either case we will
	// continue using the last valid Random Beacon key until the next spork.
	//
	currentFinalView, err := current.FinalView()
	if err != nil {
		return 0, err
	}
	if view > currentFinalView {
		_, err := next.DKG() // either of the following errors indicates that we have transitioned into EECC
		if errors.Is(err, protocol.ErrEpochNotCommitted) || errors.Is(err, protocol.ErrNextEpochNotSetup) {
			return current.Counter()
		}
		if err != nil {
			return 0, fmt.Errorf("unexpected error in EECC logic while retrieving DKG data: %w", err)
		}
	}

	// HAPPY PATH logic
	return l.EpochForView(view)
}
