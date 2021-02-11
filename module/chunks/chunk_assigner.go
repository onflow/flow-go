package chunks

import (
	"fmt"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/crypto/random"
	chunkmodels "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/indices"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/state/protocol"
)

// DefaultChunkAssignmentAlpha is the default number of verifiers that should be
// assigned to each chunk.
// DISCLAIMER: the current value is not necessarily suitable for production
const DefaultChunkAssignmentAlpha = 20

// ChunkAssigner implements an instance of the Public Chunk Assignment
// algorithm for assigning chunks to verifier nodes in a deterministic but
// unpredictable manner. It implements the ChunkAssigner interface.
type ChunkAssigner struct {
	alpha       int // used to indicate the number of verifiers that should be assigned to each chunk
	assignments mempool.Assignments

	protocolState protocol.State
}

// NewChunkAssigner generates and returns an instance of the Public Chunk
// Assignment algorithm. Parameter alpha is the number of verifiers that should
// be assigned to each chunk.
func NewChunkAssigner(alpha uint, protocolState protocol.State) (*ChunkAssigner, error) {
	// TODO to have limit of assignment mempool as a parameter (2703)
	assignment, err := stdmap.NewAssignments(1000)
	if err != nil {
		return nil, fmt.Errorf("could not create an assignment mempool: %w", err)
	}
	return &ChunkAssigner{
		alpha:         int(alpha),
		assignments:   assignment,
		protocolState: protocolState,
	}, nil
}

// Size returns number of assignments
func (p *ChunkAssigner) Size() uint {
	return p.assignments.Size()
}

// Assign generates the assignment
func (p *ChunkAssigner) Assign(result *flow.ExecutionResult, blockID flow.Identifier) (*chunkmodels.Assignment, error) {
	// computes a finger print for blockID||resultID||alpha
	hash, err := fingerPrint(blockID, result.ID(), p.alpha)
	if err != nil {
		return nil, fmt.Errorf("could not compute hash of identifiers: %w", err)
	}

	// checks cache against this assignment
	assignmentFingerprint := flow.HashToID(hash)
	a, exists := p.assignments.ByID(assignmentFingerprint)
	if exists {
		return a, nil
	}

	// Get a list of verifiers
	snapshot := p.protocolState.AtBlockID(blockID)
	verifiers, err := snapshot.Identities(filter.And(filter.HasRole(flow.RoleVerification), filter.HasStake(true)))
	if err != nil {
		return nil, fmt.Errorf("could not get verifiers: %w", err)
	}

	// create RNG for assignment
	rng, err := p.rngByBlockID(snapshot)
	if err != nil {
		return nil, err
	}

	// otherwise, it computes the assignment and caches it for future calls
	a, err = chunkAssignment(verifiers.NodeIDs(), result.Chunks, rng, p.alpha)
	if err != nil {
		return nil, fmt.Errorf("could not complete chunk assignment: %w", err)
	}

	// adds assignment to mempool
	_ = p.assignments.Add(assignmentFingerprint, a)

	return a, nil
}

func (p *ChunkAssigner) rngByBlockID(stateSnapshot protocol.Snapshot) (random.Rand, error) {
	// TODO: rng could be cached to optimize performance

	seed, err := stateSnapshot.Seed(indices.ProtocolVerificationChunkAssignment...)
	if err != nil {
		return nil, err
	}

	rng, err := random.NewRand(seed)
	if err != nil {
		return nil, fmt.Errorf("could not generate random generator: %w", err)
	}

	return rng, nil
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

func fingerPrint(blockID flow.Identifier, resultID flow.Identifier, alpha int) (hash.Hash, error) {
	hasher := hash.NewSHA3_256()

	// encodes alpha parameteer
	encAlpha, err := encoding.DefaultEncoder.Encode(alpha)
	if err != nil {
		return nil, fmt.Errorf("could not encode alpha: %w", err)
	}

	_, err = hasher.Write(blockID[:])
	if err != nil {
		return nil, fmt.Errorf("could not hash blockID: %w", err)
	}
	_, err = hasher.Write(resultID[:])
	if err != nil {
		return nil, fmt.Errorf("could not hash result: %w", err)
	}
	_, err = hasher.Write(encAlpha)
	if err != nil {
		return nil, fmt.Errorf("could not hash alpha: %w", err)
	}

	return hasher.SumHash(), nil
}

// IsValidVerifer returns true if the approver was assigned to the chunk
func IsValidVerifer(assignment *chunkmodels.Assignment, chunk *flow.Chunk, approver flow.Identifier) bool {
	verifiers := assignment.Verifiers(chunk)
	for _, verifier := range verifiers {
		if verifier == approver {
			return true
		}
	}

	return false
}
