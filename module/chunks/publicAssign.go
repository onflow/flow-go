package chunks

import (
	"fmt"
	"sort"

	"github.com/dapperlabs/flow-go/model/indices"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/crypto/random"
	chunkmodels "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/state/protocol"
)

// DefaultChunkAssignmentAlpha is the default number of verifiers that should be
// assigned to each chunk.
// DISCLAIMER: alpha down there is not a production-level value
const DefaultChunkAssignmentAlpha = 1

// PublicAssignment implements an instance of the Public Chunk Assignment
// algorithm for assigning chunks to verifier nodes in a deterministic but
// unpredictable manner. It implements the ChunkAssigner interface.
type PublicAssignment struct {
	alpha       int // used to indicate the number of verifiers that should be assigned to each chunk
	assignments mempool.Assignments

	rngByBlockID func(flow.Identifier) (random.Rand, error)
}

// NewPublicAssignment generates and returns an instance of the Public Chunk
// Assignment algorithm. Parameter alpha is the number of verifiers that should
// be assigned to each chunk.
func NewPublicAssignment(alpha int, rngByBlockID func(flow.Identifier) (random.Rand, error)) (*PublicAssignment, error) {
	// TODO to have limit of assignment mempool as a parameter (2703)
	assignment, err := stdmap.NewAssignments(1000)
	if err != nil {
		return nil, fmt.Errorf("could not create an assignment mempool: %w", err)
	}
	return &PublicAssignment{
		alpha:        alpha,
		assignments:  assignment,
		rngByBlockID: rngByBlockID,
	}, nil
}

// Size returns number of assignments
func (p *PublicAssignment) Size() uint {
	return p.assignments.Size()
}

// Assign generates the assignment
func (p *PublicAssignment) Assign(verifiers flow.IdentityList, chunks flow.ChunkList, blockID flow.Identifier) (*chunkmodels.Assignment, error) {
	// TODO: rng could be cached to optimize performance
	// create RNG for assignment
	rng, err := p.rngByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not generate random generator: %w", err)
	}

	return p.assign(verifiers, chunks, rng)
}

func (p *PublicAssignment) assign(verifiers flow.IdentityList, chunks flow.ChunkList, rng random.Rand) (*chunkmodels.Assignment, error) {
	// computes a finger print for identities||chunks
	ids := verifiers.NodeIDs()
	hash, err := fingerPrint(ids, chunks, p.alpha)
	if err != nil {
		return nil, fmt.Errorf("could not compute hash of identifiers: %w", err)
	}

	// checks cache against this assignment
	assignmentFingerprint := flow.HashToID(hash)
	a, exists := p.assignments.ByID(assignmentFingerprint)
	if exists {
		return a, nil
	}

	// otherwise, it computes the assignment and caches it for future calls
	a, err = chunkAssignment(ids, chunks, rng, p.alpha)
	if err != nil {
		return nil, fmt.Errorf("could not complete chunk assignment: %w", err)
	}

	// adds assignment to mempool
	added := p.assignments.Add(assignmentFingerprint, a)
	if !added {
		return nil, fmt.Errorf("could not add generated assignment to mempool")
	}

	return a, nil
}

// ChunkAssignment implements the business logic of the Public Chunk Assignment algorithm and returns an
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
// - result with sorted version of chunk list
// - alpha
// the generated fingerprint is deterministic in the set of aforementioned parameters
func fingerPrint(ids flow.IdentifierList, chunks flow.ChunkList, alpha int) (hash.Hash, error) {
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
	_, err = hasher.Write(encAlpha)
	if err != nil {
		return nil, fmt.Errorf("could not hash alpha: %w", err)
	}

	return hasher.SumHash(), nil
}

// CreateRNGByBlockIDClosure returns a function to get an RNG by blockID
func CreateRNGByBlockIDClosure(state protocol.State) func(flow.Identifier) (random.Rand, error) {
	return func(blockID flow.Identifier) (random.Rand, error) {
		snapshot := state.AtBlockID(blockID)
		seed, err := snapshot.Seed(indices.ProtocolVerificationChunkAssignment...)
		if err != nil {
			return nil, fmt.Errorf("could not generate seed: %v", err)
		}

		rng, err := random.NewRand(seed)
		if err != nil {
			return nil, fmt.Errorf("could not generate random generator: %w", err)
		}

		return rng, nil
	}
}
