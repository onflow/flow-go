package backdata

import (
	"encoding/binary"

	"github.com/onflow/flow-go/model/flow"
)

type arrayCache struct {
	C [bucketSize * bucketNum]cachedEntity // first-level cache
}

func newArrayCache() *arrayCache {
	return &arrayCache{}
}

// bucketIndex converts the array-level index into bucket index and element index inside
// bucket
func bucketIndex(index uint64) (uint64, uint64) {
	bIndex := index / bucketSize // bucket index
	eIndex := index % bucketSize // element index within bucket
	return bIndex, eIndex
}

func (a *arrayCache) add(entityID flow.Identifier, entity cachedEntity) bool {
	return false
}

func idToIndex(entityID flow.Identifier) uint64 {
	return binary.LittleEndian.Uint64(entityID[:64])
}
