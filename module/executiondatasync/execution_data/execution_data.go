package execution_data

import (
	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// DefaultMaxBlobSize is the default maximum size of a blob.
// This is calibrated to fit within a libp2p message and not exceed the max size recommended by bitswap.
const DefaultMaxBlobSize = 1 << 20 // 1MiB

// ChunkExecutionData represents the execution data of a chunk
type ChunkExecutionData struct {
	Collection *flow.Collection
	Events     flow.EventsList
	TrieUpdate *ledger.TrieUpdate
}

// BlockExecutionDataRoot represents the root of a serialized BlockExecutionData.
// The hash of the serialized BlockExecutionDataRoot is the ExecutionDataID used within an flow.ExecutionResult.
type BlockExecutionDataRoot struct {
	// BlockID is the ID of the block who's result this execution data is for.
	BlockID flow.Identifier

	// ChunkExecutionDataIDs is a list of the root CIDs for each serialized ChunkExecutionData
	// associated with this block.
	ChunkExecutionDataIDs []cid.Cid
}

// BlockExecutionData represents the execution data of a block.
type BlockExecutionData struct {
	BlockID             flow.Identifier
	ChunkExecutionDatas []*ChunkExecutionData
}
