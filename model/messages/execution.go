package messages

import (
	"github.com/onflow/flow-go/model/flow"
)

// ChunkDataRequest represents a request for the chunk data pack
// which is specified by a chunk ID.
type ChunkDataRequest flow.ChunkDataRequest

// ToInternal returns the internal type representation for ChunkDataRequest.
//
// All errors indicate that the decode target contains a structurally invalid representation of the internal flow.ChunkDataRequest.
func (c *ChunkDataRequest) ToInternal() (any, error) {
	return (*flow.ChunkDataRequest)(c), nil
}

// ChunkDataResponse is the response to a chunk data pack request.
// It contains the chunk data pack of the interest.
type ChunkDataResponse struct {
	ChunkDataPack flow.ChunkDataPack
	Nonce         uint64 // so that we aren't deduplicated by the network layer
}

// ToInternal converts the untrusted ChunkDataResponse into its trusted internal
// representation.
//
// This stub returns the receiver unchanged. A proper implementation
// must perform validation checks and return a constructed internal
// object.
func (c *ChunkDataResponse) ToInternal() (any, error) {
	// TODO(malleability, #7716) implement with validation checks
	return c, nil
}
