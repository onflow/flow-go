package hotstuff

// Persister is responsible for persisting state we need to bootstrap after a
// restart or crash.
type Persister interface {

	// CurrentView stores the view we are currently on.
	CurrentView(view uint64) error
}
