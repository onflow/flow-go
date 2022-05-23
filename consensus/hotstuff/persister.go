package hotstuff

// Persister is responsible for persisting state we need to bootstrap after a
// restart or crash.
type Persister interface {

	// GetStarted will retrieve the last started view.
	GetStarted() (uint64, error)

	// GetVoted will retrieve the last voted view.
	GetVoted() (uint64, error)

	// PutStarted persists the last started view.
	PutStarted(view uint64) error

	// PutVoted persists the last voted view.
	PutVoted(view uint64) error
}
