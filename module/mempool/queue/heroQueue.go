package queue

import (
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
)

// HeroQueue implements a HeroCache-based in-memory queue.
// HeroCache is a key-value cache with zero heap allocation and optimized Garbage Collection.
type HeroQueue struct {
	mu        sync.RWMutex
	cache     *herocache.Cache
	sizeLimit uint
}

func NewHeroQueue(sizeLimit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics,
) *HeroQueue {
	return &HeroQueue{
		cache: herocache.NewCache(
			sizeLimit,
			herocache.DefaultOversizeFactor,
			heropool.NoEjection,
			logger.With().Str("mempool", "hero-queue").Logger(),
			collector),
		sizeLimit: uint(sizeLimit),
	}
}

// Push stores the entity into the queue.
// Boolean returned variable determines whether push was successful, i.e.,
// push may be dropped if queue is full or already exists.
func (c *HeroQueue) Push(entity flow.Entity) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cache.Size() >= c.sizeLimit {
		// we check size before attempt on a push,
		// although HeroCache is on no-ejection mode and discards pushes beyond limit,
		// we save an id computation by just checking the size here.
		return false
	}

	return c.cache.Add(entity.ID(), entity)
}

// Pop removes and returns the head of queue, and updates the head to the next element.
// Boolean return value determines whether pop is successful, i.e., popping an empty queue returns false.
func (c *HeroQueue) Pop() (flow.Entity, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	head, ok := c.cache.Head()
	if !ok {
		// cache is empty, and there is no head yet to pop.
		return nil, false
	}

	c.cache.Remove(head.ID())
	return head, true
}

func (c *HeroQueue) Size() uint {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.cache.Size()
}
