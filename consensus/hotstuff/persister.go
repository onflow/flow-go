package hotstuff

// Persister is responsible for persisting minimal critical safety and liveness data for HotStuff:
// specifically [hotstuff.LivenessData] and [hotstuff.SafetyData].
//
// Persister depends on protocol.State and cluster.State bootstrapping to set initial values for
// SafetyData and LivenessData, for each distinct chain ID. This bootstrapping must be complete
// before constructing a Persister instance with New (otherwise it will return an error).
type Persister interface {
	// GetSafetyData will retrieve last persisted safety data.
	GetSafetyData() *SafetyData

	// PutSafetyData persists the last safety data.
	// This method blocks until `safetyData` was successfully persisted.
	// During normal operations, no errors are expected.
	PutSafetyData(safetyData *SafetyData) error

	// GetLivenessData will retrieve last persisted liveness data.
	GetLivenessData() *LivenessData

	// PutLivenessData persists the last liveness data.
	// This method blocks until `safetyData` was successfully persisted.
	// During normal operations, no errors are expected.
	PutLivenessData(livenessData *LivenessData) error
}
