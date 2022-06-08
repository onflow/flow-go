package module

import (
	chmodels "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
)

// ChunkAssigner presents an interface for assigning chunks to the verifier nodes
type ChunkAssigner interface {
	// Assign generates the assignment
	// error returns:
	//  * NoValidChildBlockError indicates that no valid child block is known
	//    (which contains the block's source of randomness)
	//  * unexpected errors should be considered symptoms of internal bugs
	Assign(result *flow.ExecutionResult, blockID flow.Identifier) (*chmodels.Assignment, error)
}

// ChunkVerifier provides functionality to verify chunks
type ChunkVerifier interface {
	// Verify verifies the given VerifiableChunk by executing it and checking the final state commitment
	// It returns a Spock Secret as a byte array, verification fault of the chunk, and an error.
	// Note: Verify should only be executed on non-system chunks. It returns an error if it is invoked on
	// system chunk.
	// TODO return challenges plus errors
	Verify(ch *verification.VerifiableChunkData) ([]byte, chmodels.ChunkFault, error)

	// SystemChunkVerify verifies a given VerifiableChunk corresponding to a system chunk.
	// by executing it and checking the final state commitment
	// It returns a Spock Secret as a byte array, verification fault of the chunk, and an error.
	// Note: Verify should only be executed on system chunks. It returns an error if it is invoked on
	// non-system chunks.
	SystemChunkVerify(ch *verification.VerifiableChunkData) ([]byte, chmodels.ChunkFault, error)
}
