package state_synchronization

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module/component"
)

// ExecutionDataReceivedCallback is a callback that is called ExecutionData is received for a new block
type ExecutionDataReceivedCallback func(*ExecutionData)

// ExecutionDataRequester is a component that syncs ExecutionData from the network, and exposes
// a callback that is called when a new ExecutionData is received
type ExecutionDataRequester interface {
	component.Component

	// OnBlockFinalized accepts block finalization notifications from the FinalizationDistributor
	OnBlockFinalized(*model.Block)

	// AddOnExecutionDataFetchedConsumer adds a callback to be called when a new ExecutionData is received
	AddOnExecutionDataFetchedConsumer(fn ExecutionDataReceivedCallback)
}
