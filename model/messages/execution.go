package messages

import (
	"github.com/onflow/flow-go/model/flow"
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
