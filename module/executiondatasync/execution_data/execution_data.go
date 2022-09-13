package execution_data

import (
	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

const DefaultMaxBlobSize = 1 << 20 // 1MiB

// ChunkExecutionData represents the execution data of a chunk
// TODO(state-sync): document how and when it is used
type ChunkExecutionData struct {
	Collection *flow.Collection
	Events     flow.EventsList
	TrieUpdate *ledger.TrieUpdate
}

// TODO(state-sync): document how and when it is used
type BlockExecutionDataRoot struct {
	BlockID               flow.Identifier
	ChunkExecutionDataIDs []cid.Cid
}

// TODO(state-sync): document how and when it is used
type BlockExecutionData struct {
	BlockID             flow.Identifier
	ChunkExecutionDatas []*ChunkExecutionData
}
