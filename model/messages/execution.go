package messages

import (
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
)

// ChunkDataRequest represents a request for the a chunk data pack
// which is specified by a chunk ID.
type ChunkDataRequest struct {
	ChunkID flow.Identifier
	Nonce   uint64 // so that we aren't deduplicated by the network layer
}

// ChunkDataResponse is the response to a chunk data pack request.
// It contains the chunk data pack of the interest.
type ChunkDataResponse struct {
	ChunkDataPack flow.ChunkDataPack
	Nonce         uint64 // so that we aren't deduplicated by the network layer
}

// ExecutionStateSyncRequest represents a request for state deltas between
// the block at the `FromHeight` and the block at the `ToHeight`
// since the state sync request only requests for sealed blocks, heights
// should be enough to specify the block deterministically.
type ExecutionStateSyncRequest struct {
	FromHeight uint64
	ToHeight   uint64
}

type ExecutionStateDelta struct {
	entity.ExecutableBlock
	StateInteractions  []*delta.Snapshot
	EndState           flow.StateCommitment
	Events             []flow.Event
	ServiceEvents      []flow.Event
	TransactionResults []flow.TransactionResult
}

func (b *ExecutionStateDelta) ID() flow.Identifier {
	return b.Block.ID()
}

func (b *ExecutionStateDelta) Checksum() flow.Identifier {
	return b.Block.Checksum()
}

func (b *ExecutionStateDelta) Height() uint64 {
	return b.Block.Header.Height
}

func (b *ExecutionStateDelta) ParentID() flow.Identifier {
	return b.Block.Header.ParentID
}
