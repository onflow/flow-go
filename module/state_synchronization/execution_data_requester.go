package state_synchronization

import (
	"github.com/onflow/flow-go/module/component"
)

// OnExecutionDataReceivedConsumer is a callback that is called ExecutionData is received for a new block
type OnExecutionDataReceivedConsumer func()

type ExecutionDataIndexedHeight interface {
	// HighestConsecutiveHeight returns the highest consecutive block height for which ExecutionData
	// has been received.
	// This method must only be called after the component is Ready. If it is called early, an error is returned.
	HighestConsecutiveHeight() uint64
}

// ExecutionDataRequester is a component that syncs ExecutionData from the network, and exposes
// a callback that is called when a new ExecutionData is received
type ExecutionDataRequester interface {
	component.Component
	ExecutionDataIndexedHeight
}
