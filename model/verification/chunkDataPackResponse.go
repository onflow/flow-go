package verification

import (
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

// ChunkDataPackResponse is an internal data structure in fetcher engine that is passed between the fetcher
// and requester engine. It conveys requested chunk data pack as well as meta-data for fetcher engine to
// process the chunk data pack.
//
//structwrite:immutable - mutations allowed only within the constructor
type ChunkDataPackResponse struct {
	chunks.Locator
	Cdp *flow.ChunkDataPack
}

// UntrustedChunkDataPackResponse is an untrusted input-only representation of a ChunkDataPackResponse,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedChunkDataPackResponse should be validated and converted into
// a trusted ChunkDataPackResponse using NewChunkDataPackResponse constructor.
type UntrustedChunkDataPackResponse ChunkDataPackResponse

// NewChunkDataPackResponse creates a new instance of ChunkDataPackResponse.
// Construction ChunkDataPackResponse allowed only within the constructor.
//
// All errors indicate a valid ChunkDataPackResponse cannot be constructed from the input.
func NewChunkDataPackResponse(untrusted UntrustedChunkDataPackResponse) (*ChunkDataPackResponse, error) {
	return &ChunkDataPackResponse{
		Locator: untrusted.Locator,
		Cdp:     untrusted.Cdp,
	}, nil
}
