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

// NewChunkDataPackResponse creates a new instance of ChunkDataPackResponse.
// Construction ChunkDataPackResponse allowed only within the constructor.
func NewChunkDataPackResponse(
	locator chunks.Locator,
	cdp *flow.ChunkDataPack,
) ChunkDataPackResponse {
	return ChunkDataPackResponse{
		Locator: locator,
		Cdp:     cdp,
	}
}
