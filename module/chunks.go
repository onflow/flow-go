package module

import (
	"github.com/onflow/flow-go/crypto/random"
	"github.com/onflow/flow-go/engine/verification"
	chmodels "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

// ChunkAssigner presents an interface for assigning chunks to the verifier nodes
type ChunkAssigner interface {
	// Assign receives identity list of verifier nodes, chunk lists and a random generator
	// it returns a chunk assignment
	Assign(ids flow.IdentityList, chunks flow.ChunkList, rng random.Rand) (*chmodels.Assignment, error)
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
