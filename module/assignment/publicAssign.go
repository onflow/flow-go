package assignment

import (
	"fmt"
	"sort"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/random"
	"github.com/dapperlabs/flow-go/model/chunkassignment"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
)

// PublicAssignment implements an instance of the Public Chunk Assignment algorithm
// for assigning chunks to verifier nodes in a deterministic but unpredictable manner.
type PublicAssignment struct {
	alpha int // used to indicate the number of verifiers should be assigned to each chunk
	cache map[string]*chunkassignment.Assignment
}

// NewPublicAssignment generates and returns an instance of the Public Chunk Assignment algorithm
// ids is the list of verifier nodes' identities
// chunks is the list of chunks aimed to assign
// rng is an instance of a random generator
// alpha is the number of assigned verifier nodes to each chunk
func NewPublicAssignment(alpha int) *PublicAssignment {
	return &PublicAssignment{
		alpha: alpha,
		cache: make(map[string]*chunkassignment.Assignment),
	}
}

// Assign receives identity list of verifier nodes, chunk lists and a random generator
// it returns a chunk assignment
func (p *PublicAssignment) Assign(identities flow.IdentityList, chunks flow.ChunkList, rng random.Rand) (*chunkassignment.Assignment, error) {
	// computes a finger print for identities||chunks
	ids := identities.NodeIDs()
	hash, err := fingerPrint(ids, chunks)
	if err != nil {
		return nil, fmt.Errorf("could not compute hash of identifiers: %w", err)
	}

	// checks cache against this assignment
	hashStr := hash.Hex()
	if a, ok := p.cache[hashStr]; ok {
		return a, nil
	}

	// otherwise, it computes the assignment and caches it for future calls
	a, err := chunkAssignment(ids, chunks, rng, p.alpha)
	if err != nil {
		return nil, errors.Wrap(err, "could not complete chunk assignment")
	}
	p.cache[hashStr] = a

	return a, nil
}

// permute shuffles subset of ids that contains its first m elements in place
// it implements in-place version of Fisher-Yates shuffling https://doi.org/10.1145%2F364520.364540
func permute(ids flow.IdentifierList, m int, rng random.Rand) {
	for i := m - 1; i > 0; i-- {
		j, _ := rng.IntN(i)
		ids.Swap(i, j)
	}
}

// chunkAssignment implements the business logic of the Public Chunk Assignment algorithm and returns an
// assignment object for the chunks where each chunk is assigned to alpha-many verifier node from ids list
func chunkAssignment(ids flow.IdentifierList, chunks flow.ChunkList, rng random.Rand, alpha int) (*chunkassignment.Assignment, error) {
	if len(ids) < alpha {
		return nil, fmt.Errorf("not enough verification nodes for chunk assignment: %d, minumum should be %d", len(ids), alpha)
	}
	assignment := chunkassignment.NewAssignment()
	// permutes the entire slice
	permute(ids, len(ids), rng)
	t := ids

	for i := 0; i < chunks.Len(); i++ {
		if len(t) >= alpha {
			// More verifiers than required for this chunk
			assignment.Add(chunks.ByIndex(uint64(i)), flow.JoinIdentifierLists(t[:alpha], nil))
			t = t[alpha:]
		} else {
			// Less verifiers than required for this chunk
			part1 := make([]flow.Identifier, len(t))
			copy(part1, t)

			still := alpha - len(t)
			permute(ids[:ids.Len()-len(t)], still, rng)

			part2 := make([]flow.Identifier, still)
			copy(part2, ids[:still])
			assignment.Add(chunks.ByIndex(uint64(i)), flow.JoinIdentifierLists(part1, part2))
			permute(ids[still:], ids.Len()-still, rng)
			t = ids[still:]
		}
	}
	return assignment, nil
}

// Fingerprint computes the SHA3-256 hash value of the sorted version of identifier list
func fingerPrint(ids flow.IdentifierList, chunks flow.ChunkList) (crypto.Hash, error) {
	// sorts and encodes ids
	sort.Sort(ids)
	encIDs, err := encoding.DefaultEncoder.Encode(ids)
	if err != nil {
		return nil, fmt.Errorf("could not encode identifier list: %w", err)
	}

	// sorts and encodes chunks
	sort.Sort(chunks)
	encChunks, err := encoding.DefaultEncoder.Encode(chunks)
	if err != nil {
		return nil, fmt.Errorf("could not encode chunk list: %w", err)
	}

	hasher, err := crypto.NewHasher(crypto.SHA3_256)
	if err != nil {
		return nil, fmt.Errorf("could not create hasher: %w", err)
	}

	// computes and returns hash(encIDs || encChunks)
	_, err = hasher.Write(encIDs)
	if err != nil {
		return nil, fmt.Errorf("could not hash ids: %w", err)
	}
	_, err = hasher.Write(encChunks)
	if err != nil {
		return nil, fmt.Errorf("could not hash chunks: %w", err)
	}

	return hasher.SumHash(), nil
}
