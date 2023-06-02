package seed

import "encoding/binary"

// list of customizers used for different sub-protocol PRNGs.
// These customizers help instantiate different PRNGs from the
// same source of randomness.

var (
	// ConsensusLeaderSelection is the customizer for consensus leader selection
	ConsensusLeaderSelection = customizerFromIndices(0, 1, 1)
	// VerificationChunkAssignment is the customizer for verification chunk assignment
	VerificationChunkAssignment = customizerFromIndices(0, 2, 0)
	// ExecutionEnvironment is the customizer for executing blocks
	ExecutionEnvironment = customizerFromIndices(1)
)

// CollectorClusterLeaderSelection returns the indices for the leader selection for the i-th collector cluster
func CollectorClusterLeaderSelection(clusterIndex uint) []byte {
	return customizerFromIndices(0, 0, uint16(clusterIndex))
}

// customizerFromIndices converts the input indices into a slice of bytes.
// The implementation ensures there are no output collisions.
//
// The output is built as a concatenation of indices, each index is encoded over 2 bytes.
func customizerFromIndices(indices ...uint16) []byte {
	customizerLen := 2 * len(indices)
	customizer := make([]byte, customizerLen)
	// concatenate the indices
	for i, index := range indices {
		binary.LittleEndian.PutUint16(customizer[2*i:2*i+2], index)
	}
	return customizer
}
