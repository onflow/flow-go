package module

import (
	"github.com/dapperlabs/flow-go/crypto/random"
	"github.com/dapperlabs/flow-go/model/chunkassignment"
	"github.com/dapperlabs/flow-go/model/flow"
)

// ChunkAssigner presents an interface for assigning chunks to the verifier nodes
type ChunkAssigner interface {
	// Assigner receives identity list of verifier nodes, chunk lists and a random generator
	// it returns a chunk assignment
	Assigner(ids flow.IdentityList, chunks flow.ChunkList, rng random.RandomGenerator) (*chunkassignment.Assignment, error)
}
