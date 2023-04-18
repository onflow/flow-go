package execution_data

import (
	"github.com/onflow/flow-go/model/flow"
)

// BlockExecutionDataEntity is a wrapper around BlockExecutionData that implements the flow.Entity
// interface to support caching with Herocache
type BlockExecutionDataEntity struct {
	*BlockExecutionData

	// id holds the cached BlockExecutionData ID. The ID generation process is expensive, so this
	// entity interface exclusively uses a pre-calculated value.
	id flow.Identifier
}

var _ flow.Entity = (*BlockExecutionDataEntity)(nil)

func NewBlockExecutionDataEntity(id flow.Identifier, executionData *BlockExecutionData) *BlockExecutionDataEntity {
	return &BlockExecutionDataEntity{
		id:                 id,
		BlockExecutionData: executionData,
	}
}

func (c BlockExecutionDataEntity) ID() flow.Identifier {
	return c.id
}

func (c BlockExecutionDataEntity) Checksum() flow.Identifier {
	return c.id
}
