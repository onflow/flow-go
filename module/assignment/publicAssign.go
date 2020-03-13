package assignment

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/crypto/random"
	"github.com/dapperlabs/flow-go/model/chunkassignment"
	"github.com/dapperlabs/flow-go/model/flow"
)

// PublicAssignment implements an instance of the Public Chunk Assignment algorithm
// for assigning chunks to verifier nodes in a deterministic but unpredictable manner.
type PublicAssignment struct {
	alpha int // used to indicate the number of verifiers should be assigned to each chunk
}

// NewPublicAssignment generates and returns an instance of the Public Chunk Assignment algorithm
// ids is the list of verifier nodes' identities
// chunks is the list of chunks aimed to assign
// rng is an instance of a random generator
// alpha is the number of assigned verifier nodes to each chunk
func NewPublicAssignment(alpha int) *PublicAssignment {
	return &PublicAssignment{alpha: alpha}
}

func (p *PublicAssignment) Assign(ids flow.IdentityList, chunks flow.ChunkList, rng random.Rand) (*chunkassignment.Assignment, error) {
	a, err := chunkAssignment(ids.NodeIDs(), chunks, rng, p.alpha)
	if err != nil {
		return nil, errors.Wrap(err, "could not complete chunk assignment")
	}

	return a, nil
}

// chunkAssignment implements the business logic of the Public Chunk Assignment algorithm and returns an
// assignment object for the chunks where each chunk is assigned to alpha-many verifier node from ids list
func chunkAssignment(ids flow.IdentifierList, chunks flow.ChunkList, rng random.Rand, alpha int) (*chunkassignment.Assignment, error) {
	if len(ids) < alpha {
		return nil, fmt.Errorf("not enough verification nodes for chunk assignment: %d, minumum should be %d", len(ids), alpha)
	}
	assignment := chunkassignment.NewAssignment()
	// permutes the entire slice
	rng.Shuffle(len(ids), ids.Swap)
	t := ids

	for i := 0; i < chunks.Size(); i++ {
		if len(t) >= alpha { // More verifiers than required for this chunk
			assignees := make([]flow.Identifier, alpha)
			copy(assignees, t[:alpha])
			assignment.Add(chunks.ByIndex(uint64(i)), assignees)
			t = t[alpha:]
		} else { // Less verifiers than required for this chunk
			// take all remaining elements from t
			part1 := make([]flow.Identifier, len(t))
			copy(part1, t)

			// now, we need `still` elements from a new shuffling round:
			still := alpha - len(part1)
			t = ids[:ids.Len()-len(part1)] // but we exclude the elements we already picked from the population
			rng.Samples(len(t), still, t.Swap)
			part2 := make([]flow.Identifier, still)
			copy(part2, t[:still])

			// concatenated part1 and part2 have exactly alpha elements and constitute the assignees for the current chunk
			assignment.Add(chunks.ByIndex(uint64(i)), flow.JoinIdentifierLists(part1, part2))

			// we have already assigned the first `still` elements in `ids`
			// note that remaining elements ids[still:] still need shuffling
			t = ids[still:]
			rng.Shuffle(len(t), t.Swap)
		}
	}
	return assignment, nil
}
