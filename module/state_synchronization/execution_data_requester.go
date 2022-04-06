package state_synchronization

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module/component"
)

// ExecutionDataReceivedCallback is a callback that is called ExecutionData is received for a new block
type ExecutionDataReceivedCallback func(*ExecutionData)

type ExecutionDataRequester interface {
	component.Component
	OnBlockFinalized(*model.Block)
	AddOnExecutionDataFetchedConsumer(fn ExecutionDataReceivedCallback)
}
