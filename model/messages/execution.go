package messages

import (
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
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

// ChunkDataPackRequest represents a request for the a chunk data pack
// which is specified by a chunk ID.
type ChunkDataPackRequest struct {
	ChunkID flow.Identifier
}

// ChunkDataPackResponse is the response to a chunk data pack request.
// It contains the chunk data pack of the interest.
type ChunkDataPackResponse struct {
	Data flow.ChunkDataPack
}

type ExecutionStateSyncRequest struct {
	CurrentBlockID flow.Identifier
	TargetBlockID  flow.Identifier
}

type ExecutionStateDelta struct {
	Block      *flow.Block
	StateViews []*delta.View
	StartState flow.StateCommitment
	EndState   flow.StateCommitment
	Events     []flow.Event
}
