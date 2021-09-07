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
// which should be all we need. In general we can't guarantee that a node will
// have access to epoch data beyond these three, so it is safe to throw an error
// for a query that doesn't fit within the view bounds of these three epochs
// (even if the node does happen to have that stored in the underlying storage)
// -- these queries indicate a bug in the querier.
func (l *EpochLookup) EpochForView(view uint64) (epochCounter uint64, err error) {
	previous := l.state.Final().Epochs().Previous()
	current := l.state.Final().Epochs().Current()
	next := l.state.Final().Epochs().Next()

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
