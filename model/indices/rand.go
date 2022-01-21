package indices

var (
	// ProtocolConsensusLeaderSelection is the indices for consensus leader selection
	ProtocolConsensusLeaderSelection = []uint32{0, 1, 1}
	// ProtocolVerificationChunkAssignment is the indices for verification nodes determines chunk assignment
	ProtocolVerificationChunkAssignment = []uint32{0, 2, 0}
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
func ProtocolCollectorClusterLeaderSelection(clusterIndex uint) []uint32 {
	return append([]uint32{0, 0}, uint32(clusterIndex))
}

// ExecutionChunk returns the indices for i-th chunk
func ExecutionChunk(chunkIndex uint32) []uint32 {
	return append([]uint32{1}, chunkIndex)
}
