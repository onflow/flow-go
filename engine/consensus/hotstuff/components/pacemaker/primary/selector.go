package primary

type ID string

// Selector determines which consensus replica is primary at a specific view
type Selector interface {
	// PrimaryAtView returns the ID of the consensus Replica which is primary at for the given view
	PrimaryAtView(view uint64) ID
}
