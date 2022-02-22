package requester

import (
	"sort"
	"sync"

	"github.com/onflow/flow-go/module/state_synchronization"
)

// executionDataCache is a cache for execution data with a maxCapacity. When reached, blocks
// with a higher height (newer) are evicted first. This is to ensure that execution data is
// available for the
type executionDataCache struct {
	maxCapacity int

	// TODO: to support non-sealed blocks, we'll need to keep track of a list of execution data
	//       for each height
	data    map[uint64]*state_synchronization.ExecutionData
	heights []uint64

	mu sync.Mutex
}

func newExecutionDataCache(maxCapacity int) *executionDataCache {
	return &executionDataCache{
		maxCapacity: maxCapacity,
		data:        make(map[uint64]*state_synchronization.ExecutionData),
	}
}

func (c *executionDataCache) Get(height uint64) (*state_synchronization.ExecutionData, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, ok := c.data[height]
	if !ok {
		return nil, false
	}

	return data, true
}

func (c *executionDataCache) Put(height uint64, data *state_synchronization.ExecutionData) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.data[height]; exists {
		// TODO: overwrite for now, to support non-sealed blocks, we'll need to add to a list
		c.data[height] = data
		return true
	}

	if len(c.heights) >= c.maxCapacity && !c.evict(height) {
		return false
	}

	c.data[height] = data
	c.heights = insert(c.heights, height)

	if len(c.heights) != len(c.data) {
		panic("execution data cache out of sync")
	}

	return true
}

func (c *executionDataCache) Delete(height uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.data, height)
	c.heights = remove(c.heights, height)

	if len(c.heights) != len(c.data) {
		panic("execution data cache out of sync")
	}

	return false
}

func (c *executionDataCache) evict(newHeight uint64) bool {
	highest := c.heights[0] // sorted in descending order
	if newHeight >= highest {
		return false
	}

	// this is a lower height, so we'll keep it and evict the highest (newest) one
	delete(c.data, highest)
	c.heights = c.heights[1:]

	return true
}

func insert(heights []uint64, h uint64) []uint64 {
	if len(heights) == 0 {
		return []uint64{h}
	}

	// find the insert position given a descending order slice
	pos := sort.Search(len(heights), func(i int) bool { return heights[i] <= h })

	switch {
	case pos < len(heights) && heights[pos] == h:
		// it already exist, skip
		return heights

	case pos == 0:
		// it's the largest, and should got in the front
		return append([]uint64{h}, heights...)

	case pos >= len(heights):
		// it's the smallest and should go on the end
		return append(heights, h)

	default:
		// place it in the correct index in the middle
		tail := append([]uint64{h}, heights[pos:]...)
		return append(heights[:pos], tail...)
	}
}

func remove(heights []uint64, h uint64) []uint64 {
	if len(heights) == 0 {
		return heights
	}

	// find the remove position given a descending order slice
	pos := sort.Search(len(heights), func(i int) bool { return heights[i] <= h })

	switch {
	case pos >= len(heights) || heights[pos] != h:
		// not in the list
		return heights

	case pos == 0:
		// remove from the front
		return heights[1:]

	case pos == len(heights)-1:
		// remove from the end
		return heights[:pos]

	default:
		// remove from the middle
		return append(heights[:pos], heights[pos+1:]...)
	}
}
