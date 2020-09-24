package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"
	lru "github.com/hashicorp/golang-lru"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
)

func withLimit(limit uint) func(*Cache) {
	return func(c *Cache) {
		c.limit = limit
	}
}

type storeFunc func(key interface{}, val interface{}) func(*badger.Txn) error

func withStore(store storeFunc) func(*Cache) {
	return func(c *Cache) {
		c.store = store
	}
}

func noStore(key interface{}, val interface{}) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		return fmt.Errorf("no store function for cache put available")
	}
}

type retrieveFunc func(key interface{}) func(*badger.Txn) (interface{}, error)

func withRetrieve(retrieve retrieveFunc) func(*Cache) {
	return func(c *Cache) {
		c.retrieve = retrieve
	}
}

func noRetrieve(key interface{}) func(*badger.Txn) (interface{}, error) {
	return func(tx *badger.Txn) (interface{}, error) {
		return nil, fmt.Errorf("no retrieve function for cache get available")
	}
}

func withResource(resource string) func(*Cache) {
	return func(c *Cache) {
		c.resource = resource
	}
}

type Cache struct {
	metrics  module.CacheMetrics
	limit    uint
	store    storeFunc
	retrieve retrieveFunc
	resource string
	cache    *lru.Cache
}

func newCache(collector module.CacheMetrics, options ...func(*Cache)) *Cache {
	c := Cache{
		metrics:  collector,
		limit:    1000,
		store:    noStore,
		retrieve: noRetrieve,
		resource: metrics.ResourceUndefined,
	}
	for _, option := range options {
		option(&c)
	}
	c.cache, _ = lru.New(int(c.limit))
	c.metrics.CacheEntries(c.resource, uint(c.cache.Len()))
	return &c
}

// Get will try to retrieve the resource from cache first, and then from the
// injected
func (c *Cache) Get(key interface{}) func(*badger.Txn) (interface{}, error) {
	return func(tx *badger.Txn) (interface{}, error) {

		// check if we have it in the cache
		resource, cached := c.cache.Get(key)
		if cached {
			c.metrics.CacheHit(c.resource)
			return resource, nil
		}

		// get it from the database
		c.metrics.CacheMiss(c.resource)
		resource, err := c.retrieve(key)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve resource: %w", err)
		}

		// cache the resource and eject least recently used one if we reached limit
		evicted := c.cache.Add(key, resource)
		if !evicted {
			c.metrics.CacheEntries(c.resource, uint(c.cache.Len()))
		}

		return resource, nil
	}
}

// Put will add an resource to the cache with the given ID.
func (c *Cache) Put(key interface{}, resource interface{}) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// try to store the resource
		err := c.store(key, resource)(tx)
		if err != nil {
			return fmt.Errorf("could not store resource: %w", err)
		}

		// cache the resource and eject least recently used one if we reached limit
		evicted := c.cache.Add(key, resource)
		if !evicted {
			c.metrics.CacheEntries(c.resource, uint(c.cache.Len()))
		}

		return nil
	}
}
