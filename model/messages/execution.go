package messages

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// ExecutionStateRequest represents a request for the portion of execution state
// used by all the transactions in the chunk specified by the chunk ID.
type ExecutionStateRequest struct {
	ChunkID flow.Identifier
}

// ExecutionStateResponse is the response to a state request. It includes all the
// registers required for the requested chunk in a concrete view.
type ExecutionStateResponse struct {
	State flow.ChunkState
}

type ExecutionStateSyncRequest struct {
	CurrentBlockID flow.Identifier
	TargetBlockID  flow.Identifier
}

type ExecutionStateDelta struct {
	BlockID flow.Identifier
	Delta   flow.RegisterDelta
}
