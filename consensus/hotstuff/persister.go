package hotstuff

// Persister is responsible for persisting state we need to bootstrap after a
// restart or crash.
type Persister interface {

	// StartedView persists the view whenever we start it.
	StartedView(view uint64) error

	// VotedView persist the view when we vote on it.
	VotedView(view uint64) error
}
