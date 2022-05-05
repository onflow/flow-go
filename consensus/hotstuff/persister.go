package hotstuff

// Persister is responsible for persisting state we need to bootstrap after a
// restart or crash.
type Persister interface {
	// GetSafetyData will retrieve last persisted safety data.
	GetSafetyData() (*SafetyData, error)

	// PutSafetyData persists the last safety data.
	PutSafetyData(safetyData *SafetyData) error

	// GetLivenessData will retrieve last persisted liveness data.
	GetLivenessData() (*LivenessData, error)

	// PutLivenessData persists the last liveness data.
	PutLivenessData(livenessData *LivenessData) error
}
