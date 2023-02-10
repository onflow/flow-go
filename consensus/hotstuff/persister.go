package hotstuff

// Persister is responsible for persisting state we need to bootstrap after a
// restart or crash.
type Persister interface {
	// GetSafetyData will retrieve last persisted safety data.
	// During normal operations, no errors are expected.
	GetSafetyData() (*SafetyData, error)

	// PutSafetyData persists the last safety data.
	// This method blocks until `safetyData` was successfully persisted.
	// During normal operations, no errors are expected.
	PutSafetyData(safetyData *SafetyData) error

	// GetLivenessData will retrieve last persisted liveness data.
	// During normal operations, no errors are expected.
	GetLivenessData() (*LivenessData, error)

	// PutLivenessData persists the last liveness data.
	// This method blocks until `safetyData` was successfully persisted.
	// During normal operations, no errors are expected.
	PutLivenessData(livenessData *LivenessData) error
}
