package indices

import "encoding/binary"

var (
	// ProtocolConsensusLeaderSelection is the indices for consensus leader selection
	ProtocolConsensusLeaderSelection = []uint16{0, 1, 1}
	// ProtocolVerificationChunkAssignment is the indices for verification nodes determines chunk assignment
	ProtocolVerificationChunkAssignment = []uint16{0, 2, 0}
)

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

// ProtocolCollectorClusterLeaderSelection returns the indices for the leader selection for the i-th collector cluster
func ProtocolCollectorClusterLeaderSelection(clusterIndex uint) []uint16 {
	return append([]uint16{0, 0}, uint16(clusterIndex))
}

// ExecutionChunk returns the indices for i-th chunk
func ExecutionChunk(chunkIndex uint16) []uint16 {
	return append([]uint16{1}, chunkIndex)
}

// customizerFromIndices maps the input indices into a slice of bytes.
// The implementation insures no collision of mapping of different indices.
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
