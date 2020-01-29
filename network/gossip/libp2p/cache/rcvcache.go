package cache

import (
	"fmt"
	"sync"

	lru "github.com/hashicorp/golang-lru"
)

// RcvCache implements an LRU cache of the received eventIDs that delivered to their engines
type RcvCache struct {
	sync.Mutex
	c *lru.Cache // used to incorporate an LRU cache
}

// RcvCacheEntry represents an entry for the RcvCache
type RcvCacheEntry struct {
	eventID   string
	channelID uint32
}

// NewRcvCache creates and returns a new RcvCache
func NewRcvCache(size int) (*RcvCache, error) {
	c, err := lru.New(size)
	if err != nil {
		return nil, fmt.Errorf("could not initialize cache %w", err)
	}

	rcv := &RcvCache{
		Mutex: sync.Mutex{},
		c:     c,
	}

	return rcv, nil
}

// Add adds an event ID and its targeted channelID to the cache
func (r *RcvCache) Add(eventID []byte, channelID uint32) {
	r.Lock()
	defer r.Unlock()
	entry := RcvCacheEntry{
		eventID:   string(eventID),
		channelID: channelID,
	}
	r.c.Add(entry, true)
}

// Seen returns true if the eventID exists in the cache for the channelID
func (r *RcvCache) Seen(eventID []byte, channelID uint32) bool {
	r.Lock()
	defer r.Unlock()
	entry := RcvCacheEntry{
		eventID:   string(eventID),
		channelID: channelID,
	}
	return r.c.Contains(entry)
}
