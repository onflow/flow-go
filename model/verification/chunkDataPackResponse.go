package verification

import (
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

// ChunkDataPackResponse is an internal data structure in fetcher engine that is passed between the fetcher
// and requester engine. It conveys requested chunk data pack as well as meta-data for fetcher engine to
// process the chunk data pack.
type ChunkDataPackResponse struct {
	chunks.Locator
	Cdp *flow.ChunkDataPack
}

func (c ChunkDataPackResponse) ID() flow.Identifier {
	return c.Locator.ID()
}

func (c ChunkDataPackResponse) Checksum() flow.Identifier {
	return c.Locator.ID()
}
