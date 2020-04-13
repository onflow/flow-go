package chunks

import (
	"fmt"
	"sort"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/crypto/random"
	chunkmodels "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
)

// PublicAssignment implements an instance of the Public Chunk Assignment algorithm
// for assigning chunks to verifier nodes in a deterministic but unpredictable manner.
type PublicAssignment struct {
	alpha       int // used to indicate the number of verifiers should be assigned to each chunk
	assignments mempool.Assignments
}

// NewPublicAssignment generates and returns an instance of the Public Chunk Assignment algorithm
// ids is the list of verifier nodes' identities
// chunks is the list of chunks aimed to assign
// rng is an instance of a random generator
// alpha is the number of assigned verifier nodes to each chunk
func NewPublicAssignment(alpha int) (*PublicAssignment, error) {
	// TODO to have limit of assignment mempool as a parameter
	// https://github.com/dapperlabs/flow-go/issues/2703
	assignment, err := stdmap.NewAssignments(1000)
	if err != nil {
		return nil, fmt.Errorf("could not create an assignment mempool: %w", err)
	}
	return &PublicAssignment{
		alpha:       alpha,
		assignments: assignment,
	}, nil
}

// Size returns number of assignments
func (p *PublicAssignment) Size() uint {
	return p.assignments.Size()
}

// Assign receives identity list of verifier nodes, chunk lists and a random generator
// it returns a chunk assignment
func (p *PublicAssignment) Assign(identities flow.IdentityList, chunks flow.ChunkList, rng random.Rand) (*chunkmodels.Assignment, error) {
	// computes a finger print for identities||chunks
	ids := identities.NodeIDs()
	hash, err := fingerPrint(ids, chunks, rng, p.alpha)
	if err != nil {
		return nil, fmt.Errorf("could not compute hash of identifiers: %w", err)
	}

	// checks cache against this assignment
	assignmentFingerprint := flow.HashToID(hash)
	if p.assignments.Has(assignmentFingerprint) {
		return p.assignments.ByID(assignmentFingerprint)
	}

	// otherwise, it computes the assignment and caches it for future calls
	a, err := chunkAssignment(ids, chunks, rng, p.alpha)
	if err != nil {
		return nil, fmt.Errorf("could not complete chunk assignment: %w", err)
	}

	// adds assignment to mempool
	err = p.assignments.Add(assignmentFingerprint, a)
	if err != nil {
		return nil, fmt.Errorf("could not add generated assignment to mempool: %w", err)
	}

	return a, nil
}

// chunkAssignment implements the business logic of the Public Chunk Assignment algorithm and returns an
// assignment object for the chunks where each chunk is assigned to alpha-many verifier node from ids list
func chunkAssignment(ids flow.IdentifierList, chunks flow.ChunkList, rng random.Rand, alpha int) (*chunkmodels.Assignment, error) {
	if len(ids) < alpha {
		return nil, fmt.Errorf("not enough verification nodes for chunk assignment: %d, minumum should be %d", len(ids), alpha)
	}

	// creates an assignment
	assignment := chunkmodels.NewAssignment()

	// permutes the entire slice
	err := rng.Shuffle(len(ids), ids.Swap)
	if err != nil {
		return nil, fmt.Errorf("shuffling verifiers failed: %w", err)
	}
	t := ids

	for i := 0; i < chunks.Len(); i++ {
		assignees := make([]flow.Identifier, 0, alpha)
		if len(t) >= alpha { // More verifiers than required for this chunk
			assignees = append(assignees, t[:alpha]...)
			t = t[alpha:]
		} else { // Less verifiers than required for this chunk
			assignees = append(assignees, t...) // take all remaining elements from t

			// now, we need `still` elements from a new shuffling round:
			still := alpha - len(assignees)
			t = ids[:ids.Len()-len(assignees)] // but we exclude the elements we already picked from the population
			err := rng.Samples(len(t), still, t.Swap)
			if err != nil {
				return nil, fmt.Errorf("sampling verifiers failed: %w", err)
			}

			// by adding `still` elements from new shuffling round: we have alpha assignees for the current chunk
			assignees = append(assignees, t[:still]...)

			// we have already assigned the first `still` elements in `ids`
			// note that remaining elements ids[still:] still need shuffling
			t = ids[still:]
			err = rng.Shuffle(len(t), t.Swap)
			if err != nil {
				return nil, fmt.Errorf("shuffling verifiers failed: %w", err)
			}
		}
		// extracts chunk by index
		chunk, ok := chunks.ByIndex(uint64(i))
		if !ok {
			return nil, fmt.Errorf("chunk out of range requested: %v", i)
		}
		assignment.Add(chunk, assignees)
	}
	return assignment, nil
}

// Fingerprint computes the SHA3-256 hash value of the inputs to the assignment algorithm:
// - sorted version of identifier list
// - sorted version of chunk list
// - internal state of random generator
// - alpha
// the generated fingerprint is deterministic in the set of aforementioned parameters
func fingerPrint(ids flow.IdentifierList, chunks flow.ChunkList, rng random.Rand, alpha int) (hash.Hash, error) {
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

	// encodes random generator
	encRng := rng.State()
	if err != nil {
		return nil, fmt.Errorf("could not encode random generator: %w", err)
	}

	// encodes alpha parameteer
	encAlpha, err := encoding.DefaultEncoder.Encode(alpha)
	if err != nil {
		return nil, fmt.Errorf("could not encode alpha: %w", err)
	}

	// computes and returns hash(encIDs || encChunks || encRng || encAlpha)
	hasher := hash.NewSHA3_256()
	_, err = hasher.Write(encIDs)
	if err != nil {
		return nil, fmt.Errorf("could not hash ids: %w", err)
	}
	_, err = hasher.Write(encChunks)
	if err != nil {
		return nil, fmt.Errorf("could not hash chunks: %w", err)
	}

	_, err = hasher.Write(encRng)
	if err != nil {
		return nil, fmt.Errorf("could not random generator: %w", err)
	}

	_, err = hasher.Write(encAlpha)
	if err != nil {
		return nil, fmt.Errorf("could not hash alpha: %w", err)
	}

	return hasher.SumHash(), nil
}
