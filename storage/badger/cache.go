package badger

import (
	"fmt"
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
)

func withLimit(limit uint) func(*Cache) {
	return func(c *Cache) {
		c.limit = limit
	}
}

type storeFunc func(flow.Identifier, interface{}) error

func withStore(store storeFunc) func(*Cache) {
	return func(c *Cache) {
		c.store = store
	}
}

func noStore(flow.Identifier, interface{}) error {
	return fmt.Errorf("no store function for cache put available")
}

type retrieveFunc func(flow.Identifier) (interface{}, error)

func withRetrieve(retrieve retrieveFunc) func(*Cache) {
	return func(c *Cache) {
		c.retrieve = retrieve
	}
}

func noRetrieve(flow.Identifier) (interface{}, error) {
	return nil, fmt.Errorf("no retrieve function for cache get available")
}

func withResource(resource string) func(*Cache) {
	return func(c *Cache) {
		c.resource = resource
	}
}

type Cache struct {
	sync.RWMutex
	metrics   module.CacheMetrics
	limit     uint
	resources map[flow.Identifier]interface{}
	store     storeFunc
	retrieve  retrieveFunc
	resource  string
}

func newCache(collector module.CacheMetrics, options ...func(*Cache)) *Cache {
	c := Cache{
		metrics:   collector,
		limit:     1000,
		resources: make(map[flow.Identifier]interface{}),
		store:     noStore,
		retrieve:  noRetrieve,
		resource:  metrics.ResourceUndefined,
	}
	for _, option := range options {
		option(&c)
	}
	return &c
}

// Get will try to retrieve the resource from cache first, and then from the
// injected
func (c *Cache) Get(entityID flow.Identifier) (interface{}, error) {

	// check if we have it in the cache
	c.RLock()
	resource, cached := c.resources[entityID]
	c.RUnlock()
	if cached {
		c.metrics.CacheHit(c.resource)
		return resource, nil
	}

	// get it from the database
	c.metrics.CacheMiss(c.resource)
	resource, err := c.retrieve(entityID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve resource: %w", err)
	}

	// cache the resource and eject a random one if we reached limit
	c.Lock()
	c.resources[entityID] = resource
	c.eject()
	c.Unlock()

	return resource, nil
}

// Put will add an resource to the cache with the given ID.
func (c *Cache) Put(entityID flow.Identifier, resource interface{}) error {

	// try to store the resource
	err := c.store(entityID, resource)
	if err != nil {
		return fmt.Errorf("could not store resource: %w", err)
	}

	// cache the resource and eject a random one if we reached limit
	c.Lock()
	c.resources[entityID] = resource
	c.eject()
	c.Unlock()

	return nil
}

// eject will check if we reached the limit and eject an resource if we did.
func (c *Cache) eject() {
	if uint(len(c.resources)) > c.limit {
		for ejectedID := range c.resources {
			delete(c.resources, ejectedID)
			break
		}
	}
	c.metrics.CacheEntries(c.resource, uint(len(c.resources)))
}
