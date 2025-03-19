package netcache

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

// ReceiveCache implements an LRU cache of the received eventIDs that delivered to their engines.
// Each key in this cache is the event ID represented as a flow.Identifier.
type ReceiveCache struct {
	*stdmap.Backend[flow.Identifier, struct{}]
}

// NewHeroReceiveCache returns a new HeroCache-based receive cache.
func NewHeroReceiveCache(sizeLimit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics,
) *ReceiveCache {
	backData := herocache.NewCache[struct{}](sizeLimit,
		herocache.DefaultOversizeFactor,
		heropool.LRUEjection, // receive cache must be LRU.
		logger.With().Str("mempool", "receive-cache").Logger(),
		collector,
	)
	backend := stdmap.NewBackend(stdmap.WithMutableBackData[flow.Identifier, struct{}](backData))
	return NewReceiveCache(uint(sizeLimit), func(cache *ReceiveCache) {
		cache.Backend = backend
	})
}

// NewReceiveCache creates and returns a new ReceiveCache
func NewReceiveCache(sizeLimit uint, opts ...func(cache *ReceiveCache)) *ReceiveCache {
	cache := &ReceiveCache{stdmap.NewBackend(stdmap.WithLimit[flow.Identifier, struct{}](sizeLimit))}

	for _, opt := range opts {
		opt(cache)
	}

	return cache
}

// Add adds a new message to the cache if not already present. Returns true if the message is new and unseen, and false if message is duplicate, and
// already has been seen by the node.
func (r *ReceiveCache) Add(eventID []byte) bool {
	return r.Backend.Add(flow.HashToID(eventID), struct{}{}) // ignore eviction status
}
