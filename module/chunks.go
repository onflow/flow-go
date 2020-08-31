package module

import (
	"github.com/dapperlabs/flow-go/crypto/random"
	"github.com/dapperlabs/flow-go/engine/verification"
	chmodels "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
)

// ChunkAssigner presents an interface for assigning chunks to the verifier nodes
type ChunkAssigner interface {
	// Assign generates the assignment
	Assign(verifiers flow.IdentityList, chunks flow.ChunkList, rng random.Rand) (*chmodels.Assignment, error)

	// AssignWithRNG generates the assignment using the execution result to seed the RNG
	AssignWithRNG(verifiers flow.IdentityList, result *flow.ExecutionResult) (*chmodels.Assignment, error)

	// GetAssignedChunks returns the list of result chunks assigned to a specifig verifier
	GetAssignedChunks(verifierID flow.Identifier, assigment *chmodels.Assignment, result *flow.ExecutionResult) (flow.ChunkList, error)
}

// ChunkVerifier provides functionality to verify chunks
type ChunkVerifier interface {
	// Verify verifies the given VerifiableChunk by executing it and checking the final state commitment
	// It returns a Spock Secret as a byte array, verification fault of the chunk, and an error.
	// Note: Verify should only be executed on non-system chunks. It returns an error if it is invoked on
	// system chunk.
	// TODO return challenges plus errors
	Verify(ch *verification.VerifiableChunkData) ([]byte, chmodels.ChunkFault, error)

	// VerifySystemChunk verifies a given VerifiableChunk corresponding to a system chunk.
	// by executing it and checking the final state commitment
	// It returns a Spock Secret as a byte array, verification fault of the chunk, and an error.
	// Note: Verify should only be executed on system chunks. It returns an error if it is invoked on
	// non-system chunks.
	SystemChunkVerify(ch *verification.VerifiableChunkData) ([]byte, chmodels.ChunkFault, error)
}
