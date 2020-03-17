package module

import (
	"github.com/dapperlabs/flow-go/crypto/random"
	"github.com/dapperlabs/flow-go/model/chunkassignment"
	"github.com/dapperlabs/flow-go/model/flow"
)

// ChunkAssigner presents an interface for assigning chunks to the verifier nodes
type ChunkAssigner interface {
	// Assign receives identity list of verifier nodes, chunk lists and a random generator
	// it returns a chunk assignment
	Assign(ids flow.IdentityList, chunks flow.ChunkList, rng random.Rand) (*chunkassignment.Assignment, error)

	// Size returns number of cached assignments in the ChunkAssigner object
	Size() uint
}
