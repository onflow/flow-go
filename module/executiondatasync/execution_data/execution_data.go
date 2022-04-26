package execution_data

import (
	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

const DefaultMaxBlobSize = 1 << 20 // 1MiB

// ChunkExecutionData represents the execution data of a chunk
type ChunkExecutionData struct {
	Collection *flow.Collection
	Events     flow.EventsList
	TrieUpdate *ledger.TrieUpdate
}

type ExecutionDataRoot struct {
	BlockID               flow.Identifier
	ChunkExecutionDataIDs []cid.Cid
}

type ExecutionData struct {
	BlockID             flow.Identifier
	ChunkExecutionDatas []*ChunkExecutionData
}
