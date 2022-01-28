package netcache

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

// RcvCache implements an LRU cache of the received eventIDs that delivered to their engines
type RcvCache struct {
	c *stdmap.Backend
}

// rcvCacheEntry represents an entry for the RcvCache
type rcvCacheEntry struct {
	eventID flow.Identifier
}

func (r rcvCacheEntry) ID() flow.Identifier {
	return r.eventID
}

func (r rcvCacheEntry) Checksum() flow.Identifier {
	return r.eventID
}

// NewRcvCache creates and returns a new RcvCache
func NewRcvCache(sizeLimit uint32, logger zerolog.Logger) *RcvCache {
	return &RcvCache{
		c: stdmap.NewBackendWithBackData(herocache.NewCache(sizeLimit,
			herocache.DefaultOversizeFactor,
			heropool.LRUEjection, // receive cache must be LRU.
			logger.With().Str("mempool", "receive-cache").Logger())),
	}
}

// Add adds a new message to the cache if not already present. Returns true if the message is new and unseen, and false if message is duplicate, and
// already has been seen by the node.
func (r *RcvCache) Add(eventID []byte) bool {
	return r.c.Add(rcvCacheEntry{eventID: flow.HashToID(eventID)}) // ignore eviction status
}
