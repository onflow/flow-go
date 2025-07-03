package optimistic_sync

// State represents the state of the processing pipeline
type State int32

const (
	// StatePending is the initial state after instantiation, before Run is called
	StatePending State = iota
	// StateDownloading represents the state where data download is in progress
	StateProcessing
	// StateWaitingPersist represents the state where all data is indexed, but conditions to persist are not met
	StateWaitingPersist
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
	case StateWaitingPersist:
		return "waiting_persist"
	case StateProcessing:
		return "processing"
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
