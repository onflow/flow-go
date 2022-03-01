package netcache

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
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

// NewReceiveCache creates and returns a new ReceiveCache
func NewReceiveCache(sizeLimit uint32, logger zerolog.Logger, metricsFactory metrics.HeroCacheMetricsRegistrationFunc) *ReceiveCache {
	return &ReceiveCache{
		c: stdmap.NewBackendWithBackData(herocache.NewCache(sizeLimit,
			herocache.DefaultOversizeFactor,
			heropool.LRUEjection, // receive cache must be LRU.
			logger.With().Str("mempool", "receive-cache").Logger(),
			metricsFactory)),
	}
}

// Add adds a new message to the cache if not already present. Returns true if the message is new and unseen, and false if message is duplicate, and
// already has been seen by the node.
func (r *ReceiveCache) Add(eventID []byte) bool {
	return r.c.Add(receiveCacheEntry{eventID: flow.HashToID(eventID)}) // ignore eviction status
}
