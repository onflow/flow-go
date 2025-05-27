package ingestion2

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	pipeline "github.com/onflow/flow-go/module/executiondatasync/optimistic_syncing"
	"github.com/onflow/flow-go/module/forest"
)

var _ forest.Vertex = (*ExecutionResultContainer)(nil)

// ExecutionResultContainer represents an ExecutionResult within the LevelledForest.
// Implements LevelledForest's Vertex interface.
type ExecutionResultContainer struct {
	result      *flow.ExecutionResult
	resultID    flow.Identifier // precomputed ID of result to avoid expensive hashing on each call
	blockHeader *flow.Header    // header of the block which the result is for
	pipeline    pipeline.Pipeline
}

// NewExecutionResultContainer instantiates an empty Equivalence Class (without any receipts)
// No errors are expected during normal operation.
func NewExecutionResultContainer(
	result *flow.ExecutionResult,
	header *flow.Header,
	pipeline pipeline.Pipeline,
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

func (c *ExecutionResultContainer) VertexID() flow.Identifier { return c.resultID }
func (c *ExecutionResultContainer) Level() uint64             { return c.blockHeader.Height }
func (c *ExecutionResultContainer) Parent() (flow.Identifier, uint64) {
	return c.result.PreviousResultID, c.blockHeader.Height - 1
}
