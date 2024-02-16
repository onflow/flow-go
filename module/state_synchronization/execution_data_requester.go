package state_synchronization

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

// OnExecutionDataReceivedConsumer is a callback that is called ExecutionData is received for a new block
type OnExecutionDataReceivedConsumer func(*execution_data.BlockExecutionDataEntity)

// ExecutionDataRequester is a component that syncs ExecutionData from the network, and exposes
// a callback that is called when a new ExecutionData is received
type ExecutionDataRequester interface {
	component.Component

	// OnBlockFinalized accepts block finalization notifications from the FollowerDistributor
	OnBlockFinalized(*model.Block)

	// HighestConsecutiveHeight returns the highest consecutive block height for which ExecutionData
	// has been received.
	// This method must only be called after the component is Ready. If it is called early, an error is returned.
	HighestConsecutiveHeight() (uint64, error)
}

// ExecutionDataIndexer is a component that indexes ExecutionData into a database
type ExecutionDataIndexer interface {
	IndexBlockData(data *execution_data.BlockExecutionDataEntity) error
}
