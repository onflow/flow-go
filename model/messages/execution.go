package messages

import (
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/model/flow"
)

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
	Block              *flow.Block
	StateInteractions  []*delta.Snapshot
	StartState         flow.StateCommitment
	EndState           flow.StateCommitment
	Events             []flow.Event
	TransactionResults []flow.TransactionResult
}

func (b *ExecutionStateDelta) ID() flow.Identifier {
	return b.Block.ID()
}

func (b *ExecutionStateDelta) Checksum() flow.Identifier {
	return b.Block.Checksum()
}

func (b *ExecutionStateDelta) Height() uint64 {
	return b.Block.Height
}

func (b *ExecutionStateDelta) ParentID() flow.Identifier {
	return b.Block.ParentID
}
