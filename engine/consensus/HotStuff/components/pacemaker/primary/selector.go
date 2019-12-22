package primary

// Selector determines which consensus replica is primary at a specific view
type Selector interface {

	// PrimaryAtView returns the index of the consensus Replica which is primary at for the given view
	PrimaryAtView(uint64) uint
}

