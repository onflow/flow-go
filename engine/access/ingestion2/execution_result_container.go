package ingestion2

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// ExecutionResultContainer represents an ExecutionResult within the LevelledForest.
// Implements LevelledForest's Vertex interface.
type ExecutionResultContainer struct {
	result      *flow.ExecutionResult
	resultID    flow.Identifier // precomputed ID of result to avoid expensive hashing on each call
	blockHeader *flow.Header    // header of the block which the result is for
	pipeline    Pipeline
}

// NewExecutionResultContainer instantiates an empty Equivalence Class (without any receipts)
func NewExecutionResultContainer(
	result *flow.ExecutionResult,
	header *flow.Header,
	pipeline Pipeline,
) (*ExecutionResultContainer, error) {
	// sanity check: initial result should be for block
	if header.ID() != result.BlockID {
		return nil, fmt.Errorf("initial result is for different block")
	}

	// construct ExecutionResultContainer only containing initialReceipt
	return &ExecutionResultContainer{
		result:      result,
		resultID:    result.ID(),
		blockHeader: header,
		pipeline:    pipeline,
	}, nil
}

/* Methods implementing LevelledForest's Vertex interface */

func (rsr *ExecutionResultContainer) VertexID() flow.Identifier { return rsr.resultID }
func (rsr *ExecutionResultContainer) Level() uint64             { return rsr.blockHeader.Height }
func (rsr *ExecutionResultContainer) Parent() (flow.Identifier, uint64) {
	return rsr.result.PreviousResultID, rsr.blockHeader.Height - 1
}
