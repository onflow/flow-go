package hotstuff

// Persister is responsible for persisting minimal critical safety and liveness data for HotStuff:
// specifically [hotstuff.LivenessData] and [hotstuff.SafetyData].
type Persister interface {
	// GetSafetyData will retrieve last persisted safety data.
	GetSafetyData() (*SafetyData, error)

	// PutSafetyData persists the last safety data.
	// This method blocks until `safetyData` was successfully persisted.
	// During normal operations, no errors are expected.
	PutSafetyData(safetyData *SafetyData) error

	// GetLivenessData will retrieve last persisted liveness data.
	GetLivenessData() (*LivenessData, error)

	// PutLivenessData persists the last liveness data.
	// This method blocks until `safetyData` was successfully persisted.
	// During normal operations, no errors are expected.
	PutLivenessData(livenessData *LivenessData) error
}
