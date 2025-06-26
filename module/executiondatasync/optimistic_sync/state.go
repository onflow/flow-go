package optimistic_sync

// State represents the state of the processing pipeline
type State int32

const (
	// StatePending is the initial state after instantiation, before Run is called
	StatePending State = iota
	// StateReady is the state after Run is called, before downloading has begun
	StateReady
	// StateDownloading represents the state where data download is in progress
	StateDownloading
	// StateIndexing represents the state where data is being indexed
	StateIndexing
	// StateWaitingPersist represents the state where all data is indexed, but conditions to persist are not met
	StateWaitingPersist
	// StatePersisting represents the state where the indexed data is being persisted to storage
	StatePersisting
	// StateComplete represents the state where all data is persisted to storage
	StateComplete
	// StateAbandoned represents the state where processing was aborted
	StateAbandoned
)

// String representation of states for logging
func (s State) String() string {
	switch s {
	case StatePending:
		return "pending"
	case StateReady:
		return "ready"
	case StateDownloading:
		return "downloading"
	case StateIndexing:
		return "indexing"
	case StateWaitingPersist:
		return "waiting_persist"
	case StatePersisting:
		return "persisting"
	case StateComplete:
		return "complete"
	case StateAbandoned:
		return "abandoned"
	default:
		return ""
	}
}

// IsTerminal returns true if the state is a terminal state (Complete or Abandoned).
func (s State) IsTerminal() bool {
	return s == StateComplete || s == StateAbandoned
}
