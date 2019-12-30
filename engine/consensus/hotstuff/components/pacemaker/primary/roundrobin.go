package primary

// RoundRobinSelector implements pacemaker.primary.selector
// Selects Primaries in Round-Robin fashion
type RoundRobinSelector struct {
	// committee of all consensus nodes for one epoch.
	committee []ID
}

func NewRoundRobinSelector(committee []ID) Selector {
	if len(committee) < 1 {
		panic("Require non-empty list of ID for the consensus committee.")
	}
	cpy := make([]ID, len(committee))
	copy(cpy, committee)
	return &RoundRobinSelector{
		committee: cpy,
	}
}

func (s *RoundRobinSelector) PrimaryAtView(viewNumber uint64) ID {
	i := int(viewNumber) % len(s.committee)
	return s.committee[i]
}



