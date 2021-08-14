package fetcher

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module"
)

// ChunkDataPackRequester encapsulates the logic of requesting a chunk data pack from an execution node.
type ChunkDataPackRequester interface {
	module.ReadyDoneAware
	// Request makes the request of chunk data pack for the specified chunk ID with the specified targets.
	Request(request *verification.ChunkDataPackRequest)
	// WithChunkDataPackHandler registers the handler component of requester engine. The handler implements the logic of handling
	// a requested chunk data pack upon its arrival.
	WithChunkDataPackHandler(handler ChunkDataPackHandler)
}

// ChunkDataPackHandler encapsulates the logic of handling a requested chunk data pack upon its arrival.
type ChunkDataPackHandler interface {
	// HandleChunkDataPack is called by the ChunkDataPackRequester anytime a new requested chunk arrives.
	// It contains the logic of handling the chunk data pack.
	HandleChunkDataPack(originID flow.Identifier, chunkDataPack *flow.ChunkDataPack)

	// NotifyChunkDataPackSealed is called by the ChunkDataPackRequester to notify the ChunkDataPackHandler that the chunk ID has been sealed and
	// hence the requester will no longer request it.
	//
	// When the requester calls this callback method, it will never returns a chunk data pack for this chunk ID to the handler (i.e.,
	// through HandleChunkDataPack).
	NotifyChunkDataPackSealed(chunkID flow.Identifier)
}
