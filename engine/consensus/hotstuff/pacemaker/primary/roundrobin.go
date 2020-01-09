package primary

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

// RoundRobinSelector implements pacemaker.primary.selector
// BASIC, NON-PRODUCTION implementation;
// Selects Primaries in Round-Robin fashion. NO support for Epochs.
type RoundRobinSelector struct {
	// committee of all consensus nodes for one epoch.
	committee []types.ID
}

func NewRoundRobinSelector(committee []types.ID) Selector {
	if len(committee) < 1 {
		panic("Require non-empty list of ID for the consensus committee.")
	}
	cpy := make([]types.ID, len(committee))
	copy(cpy, committee)
	return &RoundRobinSelector{
		committee: cpy,
	}
}

func (s *RoundRobinSelector) PrimaryAtView(viewNumber uint64) types.ID {
	i := int(viewNumber) % len(s.committee)
	return s.committee[i]
}
