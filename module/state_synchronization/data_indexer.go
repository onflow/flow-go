package state_synchronization

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

// ExecutionDataIndexer is a component that indexes ExecutionData into a database
type ExecutionDataIndexer interface {
	IndexBlockData(data *execution_data.BlockExecutionDataEntity) error
}

type CollectionHandler interface {
	HandleCollection(collection *flow.Collection) error
}
