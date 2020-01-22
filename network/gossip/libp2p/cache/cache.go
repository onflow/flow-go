package cache

import (
	"encoding/hex"
	"sync"

	"github.com/dapperlabs/flow-go/network/gossip/libp2p/message"
)

// Cache implements a naive cache for peers.
type Cache struct {
	sync.Mutex
	caches map[uint8]map[string]*message.Message
}

// New creates a new naive cache.
func New() (*Cache, error) {
	c := &Cache{
		caches: make(map[uint8]map[string]*message.Message),
	}
	return c, nil
}

// Add will add a new channel cache.
func (c *Cache) Add(channelID uint8) {
	c.Lock()
	defer c.Unlock()
	c.caches[channelID] = make(map[string]*message.Message)
}

// Has returns whether we know the given ID.
func (c *Cache) Has(channelID uint8, eventID []byte) bool {
	c.Lock()
	defer c.Unlock()
	cache := c.caches[channelID]
	key := hex.EncodeToString(eventID)
	_, ok := cache[key]
	return ok
}

// Set sets the response for the given ID.
func (c *Cache) Set(channelID uint8, eventID []byte, res *message.Message) {
	c.Lock()
	defer c.Unlock()
	cache := c.caches[channelID]
	key := hex.EncodeToString(eventID)
	cache[key] = res
}

// Get returns the payload for the given ID.
func (c *Cache) Get(channelID uint8, eventID []byte) (*message.Message, bool) {
	c.Lock()
	defer c.Unlock()
	cache := c.caches[channelID]
	key := hex.EncodeToString(eventID)
	res, ok := cache[key]
	return res, ok
}
