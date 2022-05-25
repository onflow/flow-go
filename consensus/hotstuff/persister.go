package hotstuff

// Persister is responsible for persisting state we need to bootstrap after a
// restart or crash.
type Persister interface {
	// GetSafetyData will retrieve last persisted safety data.
	GetSafetyData() (*SafetyData, error)

	// PutSafetyData persists the last safety data.
	// This method blocks until `safetyData` was successfully persisted.    
	PutSafetyData(safetyData *SafetyData) error

	// GetLivenessData will retrieve last persisted liveness data.
	GetLivenessData() (*LivenessData, error)

	// PutLivenessData persists the last liveness data.
	// This method blocks until `safetyData` was successfully persisted.	
	PutLivenessData(livenessData *LivenessData) error
}
