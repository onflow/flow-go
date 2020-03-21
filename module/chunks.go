package module

import (
	"github.com/dapperlabs/flow-go/engine/verification"
)

// ChunkVerifier provides functionality to verify chunks
type ChunkVerifier interface {
	// Verify verifies the given VerifiableChunk by executing it and checking the final statecommitment
	// TODO return challenges plus errors
	Verify(ch *verification.VerifiableChunk) error
}
