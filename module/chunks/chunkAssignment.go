package chunks

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/verification/utils"
	chunkmodels "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
)

const (
	// chunkAssignmentAlpha represents number of verification
	// DISCLAIMER: alpha down there is not a production-level value
	chunkAssignmentAlpha = 1
)

// ChunkAssignment handles all the logic related to chunk assignments
type ChunkAssignment struct {
	assigner module.ChunkAssigner
}

// NewChunkAssignment created a new Assignment struct with PublicAssignment as the assigner
func NewChunkAssignment() (*ChunkAssignment, error) {
	// we are using a public assignment scheme here
	assigner, err := NewPublicAssignment(chunkAssignmentAlpha)
	if err != nil {
		return nil, fmt.Errorf("cannot construct new public assignment")
	}

	assignment := &ChunkAssignment{
		assigner: assigner,
	}

	return assignment, nil
}

// chunks uses an assignment and a flow identity to return the chunks assigned to that identifier.
// func (ca *ChunkAssignment) chunks(myID flow.Identifier, assignment *chunkmodels.Assignment) (flow.ChunkList, error) {
// 	// indices of chunks assigned to this node
// 	chunkIndices := assignment.ByNodeID(myID)

// 	// mine keeps the list of chunks assigned to this node
// 	mine := make(flow.ChunkList, 0, len(chunkIndices))
// 	for _, index := range chunkIndices {
// 		chunk, ok := result.Chunks.ByIndex(index)
// 		if !ok {
// 			return nil, fmt.Errorf("chunk out of range requested: %v", index)
// 		}

// 		mine = append(mine, chunk)
// 	}

// 	return mine, nil
// }

// TODO: should we store the assignment for future calls? This could possible go wrong as
// other execution results are used.

// Assign generates the assignment using the execution result to seed the RNG
func (ca *ChunkAssignment) Assign(verifiers flow.IdentityList, result *flow.ExecutionResult) (*chunkmodels.Assignment, error) {
	rng, err := utils.NewChunkAssignmentRNG(result)
	if err != nil {
		return nil, fmt.Errorf("could not generate random generator: %w", err)
	}

	assignment, err := ca.assigner.Assign(verifiers, result.Chunks, rng)
	if err != nil {
		return nil, fmt.Errorf("could not create chunk assignment %w", err)
	}

	return assignment, nil
}
