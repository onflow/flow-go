package messages

import (
	"github.com/onflow/flow-go/model/flow"
)

// ChunkDataRequest represents a request for the a chunk data pack
// which is specified by a chunk ID.
//
//structwrite:immutable
type ChunkDataRequest struct {
	ChunkID flow.Identifier
	Nonce   uint64 // so that we aren't deduplicated by the network layer
}

// UntrustedChunkDataRequest is an untrusted input-only representation of a ChunkDataRequest,
// used for construction.
//
// An instance of UntrustedChunkDataRequest should be validated and converted into
// a trusted ChunkDataRequest using NewChunkDataRequest constructor.
type UntrustedChunkDataRequest ChunkDataRequest

// NewChunkDataRequest creates a new instance of ChunkDataRequest.
//
// Parameters:
//   - untrusted: untrusted ChunkDataRequest to be validated
//
// Returns:
//   - *ChunkDataRequest: the newly created instance
//   - error: any error that occurred during creation
//
// Expected Errors:
//   - TODO: add validation errors
func NewChunkDataRequest(untrusted UntrustedChunkDataRequest) (*ChunkDataRequest, error) {
	// TODO: add validation logic
	return &ChunkDataRequest{ChunkID: untrusted.ChunkID, Nonce: untrusted.Nonce}, nil
}

// ChunkDataResponse is the response to a chunk data pack request.
// It contains the chunk data pack of the interest.
//
//structwrite:immutable
type ChunkDataResponse struct {
	ChunkDataPack flow.ChunkDataPack
	Nonce         uint64 // so that we aren't deduplicated by the network layer
}

// UntrustedChunkDataResponse is an untrusted input-only representation of a ChunkDataResponse,
// used for construction.
//
// An instance of UntrustedChunkDataResponse should be validated and converted into
// a trusted ChunkDataResponse using NewChunkDataResponse constructor.
type UntrustedChunkDataResponse ChunkDataResponse

// NewChunkDataResponse creates a new instance of ChunkDataResponse.
//
// Parameters:
//   - untrusted: untrusted ChunkDataResponse to be validated
//
// Returns:
//   - *ChunkDataResponse: the newly created instance
//   - error: any error that occurred during creation
//
// Expected Errors:
//   - TODO: add validation errors
func NewChunkDataResponse(untrusted UntrustedChunkDataResponse) (*ChunkDataResponse, error) {
	// TODO: add validation logic
	return &ChunkDataResponse{ChunkDataPack: untrusted.ChunkDataPack, Nonce: untrusted.Nonce}, nil
}
