package pipeline

// State represents the state of the processing [Pipeline]
type State int

const (
	// StatePending is the initial state before the pipeline's Run method is called
	StatePending State = iota
	// StateReady is the initial state after instantiation and before downloading has begun
	StateReady
	// StateDownloading represents the state where data download is in progress
	StateDownloading
	// StateIndexing represents the state where data is being indexed
	StateIndexing
	// StateWaitingForCommit represents the state where all data is indexed, but we are still waiting
	// for the state to be committed by the protocol
	StateWaitingForCommit
	// StatePersisting indicates that the process of persisting the indexed data to storage is currently ongoing
	StatePersisting
	// StateComplete represents the state where all data is persisted to storage
	StateComplete
	// StateAbandoned indicates that the protocol has abandoned this state and hence processing was aborted
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
	case StateWaitingForCommit:
		return "waiting_commit"
	case StatePersisting:
		return "persisting"
	case StateComplete:
		return "complete"
	case StateAbandoned:
		return "canceled"
	default:
		return ""
	}
}

func (s State) IsTerminal() bool {
	return s == StateComplete || s == StateAbandoned
}
