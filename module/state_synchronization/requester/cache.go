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
	}

	if len(c.heights) >= c.maxCapacity && !c.evict(height) {
		return false
	}

	c.heights = append(c.heights, height)
	c.data[height] = data

	// sort descending
	sort.Slice(c.heights, func(i, j int) bool {
		return c.heights[i] > c.heights[j]
	})

	return true
}

func (c *executionDataCache) Delete(height uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.data, height)
	for i, h := range c.heights {
		if h == height {
			c.heights = append(c.heights[:i], c.heights[i+1:]...)
			return true
		}
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
