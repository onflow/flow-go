package hotstuff

// Persister is responsible for persisting state we need to bootstrap after a
// restart or crash.
type Persister interface {

	// StartedView stores the view we are currently on.
	StartedView(view uint64) error

	// VotedView stores the view we just voted on.
	VotedView(view uint64) error
}
