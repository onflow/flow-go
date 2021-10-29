package chunks

import (
	"github.com/onflow/flow-go/model/flow"
)

// Locator is used to locate a chunk by providing the execution result the chunk belongs to as well as the chunk index within that execution result.
// Since a chunk is unique by the result ID and its index in the result's chunk list.
type Locator struct {
	ResultID flow.Identifier // execution result id that chunk belongs to
	Index    uint64          // index of chunk in the execution result
}

// ID returns a unique id for chunk locator.
func (c Locator) ID() flow.Identifier {
	return flow.MakeID(c)
}

// Checksum provides a cryptographic commitment for a chunk locator content.
func (c Locator) Checksum() flow.Identifier {
	return flow.MakeID(c)
}

// ChunkLocatorID is a util function that returns identifier of corresponding chunk locator to
// the specified result and chunk index.
func ChunkLocatorID(resultID flow.Identifier, chunkIndex uint64) flow.Identifier {
	return Locator{
		ResultID: resultID,
		Index:    chunkIndex,
	}.ID()
}

type LocatorList []*Locator

// Contains returns whether the list already contains the chunk locator.
func (l LocatorList) Contains(resultID flow.Identifier, chunkIndex uint64) bool {
	for _, existing := range l {
		if existing.ResultID == resultID && existing.Index == chunkIndex {
			return true
		}
	}

	return false
}
