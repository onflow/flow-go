package queue

import (
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
)

// HeroQueue is a generic in-memory queue implementation based on HeroCache.
// HeroCache is a key-value cache with zero heap allocation and optimized Garbage Collection.
type HeroQueue[V any] struct {
	mu        sync.RWMutex
	cache     *herocache.Cache[V]
	sizeLimit uint
}

// NewHeroQueue creates a new instance of HeroQueue with the specified size limit.
func NewHeroQueue[V any](sizeLimit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics) *HeroQueue[V] {
	return &HeroQueue[V]{
		cache: herocache.NewCache[V](
			sizeLimit,
			herocache.DefaultOversizeFactor,
			heropool.NoEjection,
			logger.With().Str("mempool", "hero-queue").Logger(),
			collector,
		),
		sizeLimit: uint(sizeLimit),
	}
}

// Push stores the key-value pair into the queue.
// Boolean returned variable determines whether push was successful, i.e.,
// push may be dropped if queue is full or already exists.
func (c *HeroQueue[V]) Push(key flow.Identifier, value V) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cache.Size() >= c.sizeLimit {
		// we check size before attempt on a push,
		// although HeroCache is on no-ejection mode and discards pushes beyond limit,
		// we save an id computation by just checking the size here.
		return false
	}

	return c.cache.Add(key, value)
}

// Pop removes and returns the head of queue, and updates the head to the next element.
// Boolean return value determines whether pop is successful, i.e., popping an empty queue returns false.
func (c *HeroQueue[V]) Pop() (value V, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var key flow.Identifier
	key, value, ok = c.cache.Head()
	if !ok {
		// cache is empty, and there is no head yet to pop.
		return value, false
	}

	c.cache.Remove(key)
	return value, true
}

// Size returns the number of elements currently stored in the queue.
func (c *HeroQueue[V]) Size() uint {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.cache.Size()
}
