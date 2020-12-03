package p2p

import (
	"fmt"

	lru "github.com/hashicorp/golang-lru"
)

// RcvCache implements an LRU cache of the received eventIDs that delivered to their engines
type RcvCache struct {
	c *lru.Cache // used to incorporate an LRU cache
}

// RcvCacheEntry represents an entry for the RcvCache
type RcvCacheEntry struct {
	eventID   string
	channelID string
}

// newRcvCache creates and returns a new RcvCache
func newRcvCache(size int) (*RcvCache, error) {
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
func (r *RcvCache) add(eventID []byte, channelID string) bool {
	entry := RcvCacheEntry{
		eventID:   string(eventID),
		channelID: channelID,
	}
	ok, _ := r.c.ContainsOrAdd(entry, true) // ignore eviction status
	return ok
}
