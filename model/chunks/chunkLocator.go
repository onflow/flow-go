package chunks

import (
	"github.com/onflow/flow-go/model/flow"
)

// ChunkLocator is stored and maintained in ChunksQueue and used to located a chunk by providing the execution result the chunk
// belongs to as well as the chunk index within the execution result.
type ChunkLocator struct {
	ResultID flow.Identifier // execution result id that chunk belongs to
	Index    uint64          // index of chunk in the execution result
}

// ID returns a unique id for chunk locator
func (c ChunkLocator) ID() flow.Identifier {
	return flow.MakeID(c)
}

// Checksum provides a cryptographic commitment for a chunk locator content.
func (c ChunkLocator) Checksum() flow.Identifier {
	return flow.MakeID(c)
}
