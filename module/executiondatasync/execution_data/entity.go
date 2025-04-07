package execution_data

import (
	"github.com/onflow/flow-go/model/flow"
)

// BlockExecutionDataEntity is a wrapper around BlockExecutionData that holds the cached BlockExecutionData ID
type BlockExecutionDataEntity struct {
	*BlockExecutionData

	// ExecutionDataID holds the cached BlockExecutionData ID. The ID generation process is expensive, so this
	// wrapper exclusively uses a pre-calculated value.
	ExecutionDataID flow.Identifier
}

func NewBlockExecutionDataEntity(executionDataID flow.Identifier, executionData *BlockExecutionData) *BlockExecutionDataEntity {
	return &BlockExecutionDataEntity{
		ExecutionDataID:    executionDataID,
		BlockExecutionData: executionData,
	}
}
