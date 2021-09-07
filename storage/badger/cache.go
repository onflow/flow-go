package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"
	lru "github.com/hashicorp/golang-lru"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

func withLimit(limit uint) func(*Cache) {
	return func(c *Cache) {
		c.limit = limit
	}
}

type storeFunc func(key interface{}, val interface{}) func(*transaction.Tx) error

const DefaultCacheSize = uint(1000)

func withStore(store storeFunc) func(*Cache) {
	return func(c *Cache) {
		c.store = store
	}
}

func noStore(key interface{}, val interface{}) func(*transaction.Tx) error {
	return func(tx *transaction.Tx) error {
		return fmt.Errorf("no store function for cache put available")
	}
}

func noopStore(key interface{}, val interface{}) func(*transaction.Tx) error {
	return func(tx *transaction.Tx) error {
		return nil
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

type Cache struct {
	metrics  module.CacheMetrics
	limit    uint
	store    storeFunc
	retrieve retrieveFunc
	resource string
	cache    *lru.Cache
}

func newCache(collector module.CacheMetrics, resourceName string, options ...func(*Cache)) *Cache {
	c := Cache{
		metrics:  collector,
		limit:    1000,
		store:    noStore,
		retrieve: noRetrieve,
		resource: resourceName,
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
		resource, err := c.retrieve(key)(tx)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				c.metrics.CacheNotFound(c.resource)
			}
			return nil, fmt.Errorf("could not retrieve resource: %w", err)
		}

		c.metrics.CacheMiss(c.resource)

		// cache the resource and eject least recently used one if we reached limit
		evicted := c.cache.Add(key, resource)
		if !evicted {
			c.metrics.CacheEntries(c.resource, uint(c.cache.Len()))
		}

		return resource, nil
	}
}

func (c *Cache) Remove(key interface{}) {
	c.cache.Remove(key)
}

// Insert will add an resource directly to the cache with the given ID
func (c *Cache) Insert(key interface{}, resource interface{}) {
	// cache the resource and eject least recently used one if we reached limit
	evicted := c.cache.Add(key, resource)
	if !evicted {
		c.metrics.CacheEntries(c.resource, uint(c.cache.Len()))
	}
}

// PutTx will return tx which adds an resource to the cache with the given ID.
func (c *Cache) PutTx(key interface{}, resource interface{}) func(*transaction.Tx) error {
	storeOps := c.store(key, resource) // assemble DB operations to store resource (no execution)

	return func(tx *transaction.Tx) error {
		err := storeOps(tx) // execute operations to store recourse
		if err != nil {
			return fmt.Errorf("could not store resource: %w", err)
		}

		tx.OnSucceed(func() {
			c.Insert(key, resource)
		})

		return nil
	}
}
