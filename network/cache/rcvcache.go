package netcache

import (
	"fmt"

	lru "github.com/hashicorp/golang-lru"

	"github.com/onflow/flow-go/network"
)

// RcvCache implements an LRU cache of the received eventIDs that delivered to their engines
type RcvCache struct {
	c *lru.Cache // used to incorporate an LRU cache
}

// RcvCacheEntry represents an entry for the RcvCache
type RcvCacheEntry struct {
	eventID string
	channel network.Channel
}

// NewRcvCache creates and returns a new RcvCache
func NewRcvCache(size int) (*RcvCache, error) {
	c, err := lru.New(size)
	if err != nil {
		return nil, fmt.Errorf("could not initialize cache %w", err)
	}

	rcv := &RcvCache{
		c: c,
	}

	return rcv, nil
}

// Add adds a new message to the cache if not already present. Returns true if the message was already in the cache and false
// otherwise
func (r *RcvCache) Add(eventID []byte, channel network.Channel) bool {
	entry := RcvCacheEntry{
		eventID: string(eventID),
		channel: channel,
	}
	ok, _ := r.c.ContainsOrAdd(entry, true) // ignore eviction status
	return ok
}
