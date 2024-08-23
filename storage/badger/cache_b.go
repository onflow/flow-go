package badger

import (
	"errors"
	"fmt"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

func withLimitB[K comparable, V any](limit uint) func(*CacheB[K, V]) {
	return func(c *CacheB[K, V]) {
		c.limit = limit
	}
}

type storeFuncB[K comparable, V any] func(key K, val V) func(storage.BadgerReaderBatchWriter) error

func withStoreB[K comparable, V any](store storeFuncB[K, V]) func(*CacheB[K, V]) {
	return func(c *CacheB[K, V]) {
		c.store = store
	}
}

func noStoreB[K comparable, V any](_ K, _ V) func(storage.BadgerReaderBatchWriter) error {
	return func(tx storage.BadgerReaderBatchWriter) error {
		return fmt.Errorf("no store function for cache put available")
	}
}

// nolint: unused
func noopStoreB[K comparable, V any](_ K, _ V) func(storage.BadgerReaderBatchWriter) error {
	return func(tx storage.BadgerReaderBatchWriter) error {
		return nil
	}
}

type retrieveFuncB[K comparable, V any] func(key K) func(storage.Reader) (V, error)

func withRetrieveB[K comparable, V any](retrieve retrieveFuncB[K, V]) func(*CacheB[K, V]) {
	return func(c *CacheB[K, V]) {
		c.retrieve = retrieve
	}
}

func noRetrieveB[K comparable, V any](_ K) func(storage.Reader) (V, error) {
	return func(tx storage.Reader) (V, error) {
		var nullV V
		return nullV, fmt.Errorf("no retrieve function for cache get available")
	}
}

type CacheB[K comparable, V any] struct {
	metrics  module.CacheMetrics
	limit    uint
	store    storeFuncB[K, V]
	retrieve retrieveFuncB[K, V]
	resource string
	cache    *lru.Cache[K, V]
}

func newCacheB[K comparable, V any](collector module.CacheMetrics, resourceName string, options ...func(*CacheB[K, V])) *CacheB[K, V] {
	c := CacheB[K, V]{
		metrics:  collector,
		limit:    1000,
		store:    noStoreB[K, V],
		retrieve: noRetrieveB[K, V],
		resource: resourceName,
	}
	for _, option := range options {
		option(&c)
	}
	c.cache, _ = lru.New[K, V](int(c.limit))
	c.metrics.CacheEntries(c.resource, uint(c.cache.Len()))
	return &c
}

// IsCached returns true if the key exists in the cache.
// It DOES NOT check whether the key exists in the underlying data store.
func (c *CacheB[K, V]) IsCached(key K) bool {
	return c.cache.Contains(key)
}

// Get will try to retrieve the resource from cache first, and then from the
// injected. During normal operations, the following error returns are expected:
//   - `storage.ErrNotFound` if key is unknown.
func (c *CacheB[K, V]) Get(key K) func(storage.Reader) (V, error) {
	return func(r storage.Reader) (V, error) {

		// check if we have it in the cache
		resource, cached := c.cache.Get(key)
		if cached {
			c.metrics.CacheHit(c.resource)
			return resource, nil
		}

		// get it from the database
		resource, err := c.retrieve(key)(r)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				c.metrics.CacheNotFound(c.resource)
			}
			var nullV V
			return nullV, fmt.Errorf("could not retrieve resource: %w", err)
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

func (c *CacheB[K, V]) Remove(key K) {
	c.cache.Remove(key)
}

// Insert will add a resource directly to the cache with the given ID
func (c *CacheB[K, V]) Insert(key K, resource V) {
	// cache the resource and eject least recently used one if we reached limit
	evicted := c.cache.Add(key, resource)
	if !evicted {
		c.metrics.CacheEntries(c.resource, uint(c.cache.Len()))
	}
}

// PutTx will return tx which adds a resource to the cache with the given ID.
func (c *CacheB[K, V]) PutTx(key K, resource V) func(storage.BadgerReaderBatchWriter) error {
	storeOps := c.store(key, resource) // assemble DB operations to store resource (no execution)

	return func(tx storage.BadgerReaderBatchWriter) error {
		tx.AddCallback(func(err error) {
			if err != nil {
				c.Insert(key, resource)
			}
		})

		err := storeOps(tx) // execute operations to store resource
		if err != nil {
			return fmt.Errorf("could not store resource: %w", err)
		}

		return nil
	}
}
