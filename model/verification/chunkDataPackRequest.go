package verification

import (
	"github.com/onflow/flow-go/model/flow"
)

// ChunkDataPackRequest is an internal data structure in fetcher engine that is passed between the engine
// and requester module. It conveys required information for requesting a chunk data pack.
type ChunkDataPackRequest struct {
	ChunkID   flow.Identifier
	Height    uint64            // block height of execution result of the chunk, used to drop chunk requests of sealed heights.
	Agrees    []flow.Identifier // execution node ids that generated the result of chunk.
	Disagrees []flow.Identifier // execution node ids that generated a conflicting result with result of chunk.
}
