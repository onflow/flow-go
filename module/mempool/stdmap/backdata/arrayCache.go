package backdata

type arrayCache struct {
	C [bucketSize * bucketNum]cachedEntity // first-level cache
}

func newArrayCache() *arrayCache {
	return &arrayCache{}
}

// bucketIndex converts the array-level index into bucket index and element index inside
// bucket
func (a arrayCache) bucketIndex(index uint) (uint, uint) {
	bIndex := index / bucketSize // bucket index
	eIndex := index % bucketSize // element index within bucket
	return bIndex, eIndex
}
