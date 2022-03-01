package state_streamer

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/state_synchronization"
)

type StateStreamer interface {
	component.Component

	// RegisterBlockConsumer registers a callback that will be called for every new
	// finalized block. The callback will be called in the order of finalization.
	RegisterBlockConsumer(FinalizedBlockConsumer)

	// RegisterExecutionDataConsumer registers a callback that will be called with the
	// Execution Data for every executed block. The callback will be called in the order
	// that blocks are sealed.
	RegisterExecutionDataConsumer(ExecutionDataConsumer)
}

type FinalizedBlockConsumer func(*flow.Block)

type ExecutionDataConsumer func(*state_synchronization.ExecutionData)

// TODO: this is a stub which needs to be implemented
type stateStreamer struct {
}

func NewStateStreamer(serverAddr string, startHeight uint64) *stateStreamer {
	return &stateStreamer{}
}
