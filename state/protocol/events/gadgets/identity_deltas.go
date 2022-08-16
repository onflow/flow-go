package gadgets

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/events"
)

// IdentityDeltas is a protocol events consumer that provides an interface to
// subscribe to callbacks any time an identity table change (or possible change)
// is finalized.
//
// TODO add slashing/ejection events here once implemented
type IdentityDeltas struct {
	events.Noop
	callback func()
}

// NewIdentityDeltas returns a new IdentityDeltas events gadget.
func NewIdentityDeltas(cb func()) *IdentityDeltas {
	deltas := &IdentityDeltas{
		callback: cb,
	}
	return deltas
}

func (g *IdentityDeltas) EpochTransition(_ uint64, _ *flow.Header) {
	g.callback()
}

func (g *IdentityDeltas) EpochSetupPhaseStarted(_ uint64, _ *flow.Header) {
	g.callback()
}

func (g *IdentityDeltas) EpochCommittedPhaseStarted(_ uint64, _ *flow.Header) {
	g.callback()
}
