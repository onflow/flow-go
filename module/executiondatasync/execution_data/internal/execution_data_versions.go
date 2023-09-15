package internal

import (
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// This is a collection of data structures from previous versions of ExecutionData.
// They are maintained here for backwards compatibility testing.
//
// Note: the current codebase makes no guarantees about backwards compatibility with previous of
// execution data. The data structures and tests included are only to help inform of any breaking
// changes.

// ChunkExecutionDataV1 [deprecated] only use for backwards compatibility testing
// was used up to block X (TODO: fill in block number after release)
type ChunkExecutionDataV1 struct {
	Collection *flow.Collection
	Events     flow.EventsList
	TrieUpdate *ledger.TrieUpdate
}
