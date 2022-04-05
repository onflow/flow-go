package state_synchronization

import (
	"github.com/ipfs/go-cid"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

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
