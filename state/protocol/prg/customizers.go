package prg

import (
	"encoding/binary"
	"fmt"
	"math"
)

// List of customizers used for different sub-protocol PRGs.
// These customizers help instantiate different PRGs from the
// same source of randomness.
//
// Customizers used by the Flow protocol should not be equal or
// prefixing each other to guarantee independent PRGs. This
// is enforced by test `TestProtocolConstants` in `./prg_test.go`

var (
	// ConsensusLeaderSelection is the customizer for consensus leader selection
	ConsensusLeaderSelection = customizerFromIndices(0)
	// VerificationChunkAssignment is the customizer for verification chunk assignment
	VerificationChunkAssignment = customizerFromIndices(2)
	// ExecutionEnvironment is the customizer for Flow's transaction execution environment
	// (used for Cadence `random` function)
	ExecutionEnvironment = customizerFromIndices(1, 0)
	// ExecutionRandomSourceHistory is the customizer for Flow's transaction execution environment
	// (used for the source of randomness history core-contract)
	ExecutionRandomSourceHistory = customizerFromIndices(1, 1)
	//
	// clusterLeaderSelectionPrefix is the prefix used for CollectorClusterLeaderSelection
	clusterLeaderSelectionPrefix = []uint16{3}
)

// CollectorClusterLeaderSelection returns the indices for the leader selection for the i-th collector cluster
func CollectorClusterLeaderSelection(clusterIndex uint) []byte {
	if uint(math.MaxUint16) < clusterIndex {
		// sanity check to guarantee no overflows during type conversion -- this should never happen
		panic(fmt.Sprintf("input cluster index (%d) exceeds max uint16 value %d", clusterIndex, math.MaxUint16))
	}
	indices := append(clusterLeaderSelectionPrefix, uint16(clusterIndex))
	return customizerFromIndices(indices...)
}

// customizerFromIndices converts the input indices into a slice of bytes.
// The function has to be injective (no different indices map to the same customizer)
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
