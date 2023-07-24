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

type storeFuncIFace func(key interface{}, val interface{}) func(*transaction.Tx) error

func withStoreIFace(store storeFuncIFace) func(face *CacheIFace) {
	return func(c *CacheIFace) {
		c.store = store
	}
}

func noStoreIFace(key interface{}, val interface{}) func(*transaction.Tx) error {
	return func(tx *transaction.Tx) error {
		return fmt.Errorf("no store function for cache put available")
	}
}

type retrieveFuncIFace func(key interface{}) func(*badger.Txn) (interface{}, error)

func noRetrieveIFace(key interface{}) func(*badger.Txn) (interface{}, error) {
	return func(tx *badger.Txn) (interface{}, error) {
		return nil, fmt.Errorf("no retrieve function for cache get available")
	}
}

type CacheIFace struct {
	metrics  module.CacheMetrics
	limit    uint
	store    storeFuncIFace
	retrieve retrieveFuncIFace
	resource string
	cache    *lru.Cache
}

func newCacheIFace(collector module.CacheMetrics, resourceName string, options ...func(*CacheIFace)) *CacheIFace {
	c := CacheIFace{
		metrics:  collector,
		limit:    1000,
		store:    noStoreIFace,
		retrieve: noRetrieveIFace,
		resource: resourceName,
	}
	for _, option := range options {
		option(&c)
	}
	c.cache, _ = lru.New(int(c.limit))
	c.metrics.CacheEntries(c.resource, uint(c.cache.Len()))
	return &c
}

// IsCached returns true if the key exists in the cache.
// It DOES NOT check whether the key exists in the underlying data store.
func (c *CacheIFace) IsCached(key any) bool {
	exists := c.cache.Contains(key)
	return exists
}

// Get will try to retrieve the resource from cache first, and then from the
// injected. During normal operations, the following error returns are expected:
//   - `storage.ErrNotFound` if key is unknown.
func (c *CacheIFace) Get(key interface{}) func(*badger.Txn) (interface{}, error) {
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

func (c *CacheIFace) Remove(key interface{}) {
	c.cache.Remove(key)
}

// Insert will add a resource directly to the cache with the given ID
func (c *CacheIFace) Insert(key interface{}, resource interface{}) {
	// cache the resource and eject least recently used one if we reached limit
	evicted := c.cache.Add(key, resource)
	if !evicted {
		c.metrics.CacheEntries(c.resource, uint(c.cache.Len()))
	}
}

// PutTx will return tx which adds a resource to the cache with the given ID.
func (c *CacheIFace) PutTx(key interface{}, resource interface{}) func(*transaction.Tx) error {
	storeOps := c.store(key, resource) // assemble DB operations to store resource (no execution)

	return func(tx *transaction.Tx) error {
		err := storeOps(tx) // execute operations to store resource
		if err != nil {
			return fmt.Errorf("could not store resource: %w", err)
		}

		tx.OnSucceed(func() {
			c.Insert(key, resource)
		})

		return nil
	}
}
