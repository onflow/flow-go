package fetcher

import (
	"github.com/onflow/flow-go/model/flow"
)

// ChunkDataPackRequester encapsulates the logic of requesting a chunk data pack from an execution node.
type ChunkDataPackRequester interface {
	// Request makes the request of chunk data pack for the specified chunk id.
	Request(chunkID flow.Identifier, executorID flow.Identifier) error
}

// ChunkDataPackHandler encapsulates the logic of handling a requested chunk data pack upon its arrival.
type ChunkDataPackHandler interface {
	// HandleChunkDataPack is called by the ChunkDataPackRequester anytime a new requested chunk arrives.
	// It contains the logic of handling the chunk data pack.
	HandleChunkDataPack(originID flow.Identifier, chunkDataPack *flow.ChunkDataPack, collection *flow.Collection) error
}
