package module

import (
	"github.com/dapperlabs/flow-go/crypto/random"
	"github.com/dapperlabs/flow-go/engine/verification"
	chmodels "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
)

// ChunkAssigner presents an interface for assigning chunks to the verifier nodes
type ChunkAssigner interface {
	// Assign receives identity list of verifier nodes, chunk lists and a random generator
	// it returns a chunk assignment
	Assign(ids flow.IdentityList, chunks flow.ChunkList, rng random.Rand) (*chmodels.Assignment, error)
}

// ChunkVerifier provides functionality to verify chunks
type ChunkVerifier interface {
	// Verify verifies the given VerifiableChunk by executing it and checking the final statecommitment
	// TODO return challenges plus errors
	Verify(ch *verification.VerifiableChunkData) (chmodels.ChunkFault, error)
}
