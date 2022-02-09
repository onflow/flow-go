package indices

import "encoding/binary"

var (
	// ProtocolConsensusLeaderSelection is the cuztomizer for consensus leader selection
	ProtocolConsensusLeaderSelection = customizerFromIndices([]uint16{0, 1, 1})
	// ProtocolVerificationChunkAssignment is the cuztomizer for verification nodes determines chunk assignment
	ProtocolVerificationChunkAssignment = customizerFromIndices([]uint16{0, 2, 0})
	// collectorClusterLeaderSelectionPrefix is the prefix of the cuztomizer for the leader selection of collector clusters
	collectorClusterLeaderSelectionPrefix = []uint16{0, 0}
	// executionChunkPrefix is the prefix of the cuztomizer for executing chunks
	executionChunkPrefix = []uint16{1}
)

// ProtocolCollectorClusterLeaderSelection returns the indices for the leader selection for the i-th collector cluster
func ProtocolCollectorClusterLeaderSelection(clusterIndex uint) []byte {
	indices := append(collectorClusterLeaderSelectionPrefix, uint16(clusterIndex))
	return customizerFromIndices(indices)
}

// ExecutionChunk returns the indices for i-th chunk
func ExecutionChunk(chunkIndex uint16) []byte {
	indices := append(executionChunkPrefix, chunkIndex)
	return customizerFromIndices(indices)
}

// list of customizers used for different sub-protocol PRNGs.
// These customizers help instanciate different PRNGs from the
// same source of randomness.
//
// TODO: the seed input is already diversified using the indices above.
// The customizers below are enough to diversify the PRNGs and we can
// remove the indices.
var (
	ConsensusLeaderSelectionCustomizer = []byte("leader_selec")
	ChunkAssignmentCustomizer          = []byte("chunk_assign")
)

// customizerFromIndices maps the input indices into a slice of bytes.
// The implementation insures there are no collisions of mapping of different indices.
//
// The output is built as a concatenation of indices, each index encoded over 2 bytes.
// (the implementation could be updated to map the indices differently depending on the
// constraints over the output length)
func customizerFromIndices(indices []uint16) []byte {
	customizerLen := 2 * len(indices)
	customizer := make([]byte, customizerLen)
	// concatenate the indices
	for i, index := range indices {
		binary.LittleEndian.PutUint16(customizer[2*i:2*i+2], index)
	}
	return customizer
}
