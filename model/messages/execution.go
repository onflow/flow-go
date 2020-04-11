package messages

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// ChunkDataPackRequest represents a request for the a chunk data pack
// which is specified by a chunk ID.
type ChunkDataPackRequest struct {
	ChunkID flow.Identifier
}

// ChunkDataPackResponse is the response to a chunk data pack request.
// It contains the chunk data pack of the interest.
type ChunkDataPackResponse struct {
	Data flow.ChunkDataPack
}
