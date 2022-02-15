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

// LocatorMap maps keeps chunk locators based on their locator id.
type LocatorMap map[flow.Identifier]*Locator

func (l LocatorMap) ToList() LocatorList {
	locatorList := LocatorList{}
	for _, locator := range l {
		locatorList = append(locatorList, locator)
	}

	return locatorList
}

type LocatorList []*Locator

func (l LocatorList) ToMap() LocatorMap {
	locatorMap := make(LocatorMap)
	for _, locator := range l {
		locatorMap[locator.ID()] = locator
	}
	return locatorMap
}
