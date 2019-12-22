package primary

// RoundRobinSelector implements pacemaker.primary.selector
// Selects Primaries in Round-Robin fashion
type RoundRobinSelector struct {
	committeeSize uint64
}

func NewRoundRobinSelector(committeeSize uint) *RoundRobinSelector {
	return &RoundRobinSelector{
		committeeSize: uint64(committeeSize),
	}
}

func (s *RoundRobinSelector) PrimaryAtView(viewNumber uint64) uint {
	return uint(viewNumber % s.committeeSize)
}

