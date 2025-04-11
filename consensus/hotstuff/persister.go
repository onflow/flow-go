package hotstuff

// Persister is responsible for persisting minimal critical safety and liveness data for HotStuff:
// specifically [hotstuff.LivenessData] and [hotstuff.SafetyData].
type Persister interface {
	PersisterReader

	// PutSafetyData persists the last safety data.
	// This method blocks until `safetyData` was successfully persisted.
	// During normal operations, no errors are expected.
	PutSafetyData(safetyData *SafetyData) error

	// PutLivenessData persists the last liveness data.
	// This method blocks until `safetyData` was successfully persisted.
	// During normal operations, no errors are expected.
	PutLivenessData(livenessData *LivenessData) error
}

// PersisterReader exposes only the read-only parts of the Persister component.
// This is used to read information about the HotStuff instance's current state from other components.
// CAUTION: the write functions are hidden here, because it is NOT SAFE to use them outside the Hotstuff state machine.
type PersisterReader interface {
	// GetSafetyData will retrieve last persisted safety data.
	// During normal operations, no errors are expected.
	GetSafetyData() (*SafetyData, error)

	// GetLivenessData will retrieve last persisted liveness data.
	// During normal operations, no errors are expected.
	GetLivenessData() (*LivenessData, error)
}
