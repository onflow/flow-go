package messages

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// ChunkDataRequest represents a request for the a chunk data pack
// which is specified by a chunk ID.
//
type ChunkDataRequest struct {
	ChunkID flow.Identifier
	Nonce   uint64 // so that we aren't deduplicated by the network layer
}

// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
type UntrustedChunkDataRequest ChunkDataRequest

//
func NewChunkDataRequest(untrusted UntrustedChunkDataRequest) (*ChunkDataRequest, error) {
	if untrusted.ChunkID == flow.ZeroID {
		return nil, fmt.Errorf("chunk ID must not be zero")
	}
	return &ChunkDataRequest{
		ChunkID: untrusted.ChunkID,
		Nonce:   untrusted.Nonce,
	}, nil
}

// ChunkDataResponse is the response to a chunk data pack request.
// It contains the chunk data pack of the interest.
//
type ChunkDataResponse struct {
	ChunkDataPack flow.ChunkDataPack
	Nonce         uint64 // so that we aren't deduplicated by the network layer
}

// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
type UntrustedChunkDataResponse ChunkDataResponse

//
func NewChunkDataResponse(untrusted UntrustedChunkDataResponse) (*ChunkDataResponse, error) {
	if untrusted.ChunkDataPack.ChunkID == flow.ZeroID {
		return nil, fmt.Errorf("chunk data pack chunk ID must not be zero")
	}
	return &ChunkDataResponse{
		ChunkDataPack: untrusted.ChunkDataPack,
		Nonce:         untrusted.Nonce,
	}, nil
}
