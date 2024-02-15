package netcache

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

// ReceiveCache implements an LRU cache of the received eventIDs that delivered to their engines
type ReceiveCache struct {
	c *stdmap.Backend
}

// receiveCacheEntry represents an entry for the ReceiveCache
type receiveCacheEntry struct {
	eventID flow.Identifier
}

func (r receiveCacheEntry) ID() flow.Identifier {
	return r.eventID
}

func (r receiveCacheEntry) Checksum() flow.Identifier {
	return r.eventID
}

// NewHeroReceiveCache returns a new HeroCache-based receive cache.
func NewHeroReceiveCache(sizeLimit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics,
) *ReceiveCache {
	backData := herocache.NewCache(sizeLimit,
		herocache.DefaultOversizeFactor,
		heropool.LRUEjection, // receive cache must be LRU.
		logger.With().Str("mempool", "receive-cache").Logger(),
		collector)
	backend := stdmap.NewBackend(stdmap.WithBackData(backData))
	return NewReceiveCache(uint(sizeLimit), func(cache *ReceiveCache) {
		cache.c = backend
	})
}

// NewReceiveCache creates and returns a new ReceiveCache
func NewReceiveCache(sizeLimit uint, opts ...func(cache *ReceiveCache)) *ReceiveCache {
	cache := &ReceiveCache{
		c: stdmap.NewBackend(stdmap.WithLimit(sizeLimit)),
	}

	for _, opt := range opts {
		opt(cache)
	}

	return cache
}

// Add adds a new message to the cache if not already present. Returns true if the message is new and unseen, and false if message is duplicate, and
// already has been seen by the node.
func (r *ReceiveCache) Add(eventID []byte) bool {
	return r.c.Add(receiveCacheEntry{eventID: flow.HashToID(eventID)}) // ignore eviction status
}

func (r *ReceiveCache) Size() uint {
	return r.c.Size()
}
