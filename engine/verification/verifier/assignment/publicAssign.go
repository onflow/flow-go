package assignment

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/crypto/random"
	"github.com/dapperlabs/flow-go/model/flow"
)

// PublicAssignment implements an instance of the Public Chunk Assignment algorithm
// for assigning chunks to verifier nodes in a deterministic but unpredictable manner.
type PublicAssignment struct {
	assignment *Assignment // used to keep assignment chunk assignment
}

// NewPublicAssignment generates and returns an instance of the Public Chunk Assignment algorithm
// ids is the list of verifier nodes' identities
// chunks is the list of chunks aimed to assign
// rng is an instance of a random generator
// alpha is the number of assigned verifier nodes to each chunk
func NewPublicAssignment(ids flow.IdentityList,
	chunks flow.ChunkList,
	rng random.Rand,
	alpha int) (*PublicAssignment, error) {

	a, err := chunkAssignment(ids.NodeIDs(), chunks, rng, alpha)
	if err != nil {
		return nil, errors.Wrap(err, "could not complete chunk assignment")
	}

	return &PublicAssignment{
		assignment: a,
	}, nil

}

// Which returns the list of chunks that are assigned to this verifier node
func (p *PublicAssignment) Which(this flow.Identifier) []uint64 {
	return p.assignment.Which(this)
}

// permute shuffles subset of ids that contains its first m elements in place
// it implements in-place version of Fisher-Yates shuffling https://doi.org/10.1145%2F364520.364540
func permute(ids IdentifierList, m int, rng random.Rand) {
	for i := m - 1; i > 0; i-- {
		j, _ := rng.IntN(i)
		ids.Swap(i, j)
	}
}

// chunkAssignment implements the business logic of the Public Chunk Assignment algorithm and returns an
// assignment object for the chunks where each chunk is assigned to alpha-many verifier node from ids list
func chunkAssignment(ids IdentifierList, chunks flow.ChunkList, rng random.Rand, alpha int) (*Assignment, error) {
	if len(ids) < alpha {
		return nil, fmt.Errorf("not enough verification nodes for chunk assignment: %d, minumum should be %d", len(ids), alpha)
	}
	assignment := NewAssignmet()
	// permutes the entire slice
	permute(ids, len(ids), rng)
	t := ids

	for i := 0; i < chunks.Size(); i++ {
		if len(t) >= alpha {
			// More verifiers than required for this chunk
			assignment.Assign(chunks.ByIndex(uint64(i)), JoinIdentifierLists(t[:alpha], nil))
			t = t[alpha:]
		} else {
			// Less verifiers than required for this chunk
			part1 := make([]flow.Identifier, len(t))
			copy(part1, t)

			still := alpha - len(t)
			permute(ids[:ids.Len()-len(t)], still, rng)

			part2 := make([]flow.Identifier, still)
			copy(part2, ids[:still])
			assignment.Assign(chunks.ByIndex(uint64(i)), JoinIdentifierLists(part1, part2))
			permute(ids[still:], ids.Len()-still, rng)
			t = ids[still:]
		}
	}
	return assignment, nil
}
