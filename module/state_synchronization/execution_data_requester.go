package state_synchronization

import (
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

// OnExecutionDataReceivedConsumer is a callback that is called ExecutionData is received for a new block
type OnExecutionDataReceivedConsumer func(*execution_data.BlockExecutionDataEntity)

type ExecutionDataIndexedHeight interface {
	// HighestConsecutiveHeight returns the highest consecutive block height for which ExecutionData
	// has been received.
	HighestConsecutiveHeight() uint64
}

// ExecutionDataRequester is a component that syncs ExecutionData from the network, and exposes
// a callback that is called when a new ExecutionData is received
type ExecutionDataRequester interface {
	component.Component
	ExecutionDataIndexedHeight
}
