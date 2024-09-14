package store

import (
	"errors"
	"fmt"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

// nolint:unused
func withLimit[K comparable, V any](limit uint) func(*Cache[K, V]) {
	return func(c *Cache[K, V]) {
		c.limit = limit
	}
}

type storeFunc[K comparable, V any] func(key K, val V) func(storage.BaseReaderBatchWriter) error

// nolint:unused
func withStore[K comparable, V any](store storeFunc[K, V]) func(*Cache[K, V]) {
	return func(c *Cache[K, V]) {
		c.store = store
	}
}

// nolint:unused
func noStore[K comparable, V any](_ K, _ V) func(storage.BaseReaderBatchWriter) error {
	return func(tx storage.BaseReaderBatchWriter) error {
		return fmt.Errorf("no store function for cache put available")
	}
}

// nolint: unused
func noopStore[K comparable, V any](_ K, _ V) func(storage.BaseReaderBatchWriter) error {
	return func(tx storage.BaseReaderBatchWriter) error {
		return nil
	}
}

type retrieveFunc[K comparable, V any] func(key K) func(storage.Reader) (V, error)

// nolint:unused
func withRetrieve[K comparable, V any](retrieve retrieveFunc[K, V]) func(*Cache[K, V]) {
	return func(c *Cache[K, V]) {
		c.retrieve = retrieve
	}
}

// nolint:unused
func noRetrieve[K comparable, V any](_ K) func(storage.Reader) (V, error) {
	return func(tx storage.Reader) (V, error) {
		var nullV V
		return nullV, fmt.Errorf("no retrieve function for cache get available")
	}
}

type Cache[K comparable, V any] struct {
	metrics module.CacheMetrics
	// nolint:unused
	limit    uint
	store    storeFunc[K, V]
	retrieve retrieveFunc[K, V]
	resource string
	cache    *lru.Cache[K, V]
}

// nolint:unused
func newCache[K comparable, V any](collector module.CacheMetrics, resourceName string, options ...func(*Cache[K, V])) *Cache[K, V] {
	c := Cache[K, V]{
		metrics:  collector,
		limit:    1000,
		store:    noStore[K, V],
		retrieve: noRetrieve[K, V],
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
func (c *Cache[K, V]) IsCached(key K) bool {
	return c.cache.Contains(key)
}

// Get will try to retrieve the resource from cache first, and then from the
// injected. During normal operations, the following error returns are expected:
//   - `storage.ErrNotFound` if key is unknown.
func (c *Cache[K, V]) Get(key K) func(storage.Reader) (V, error) {
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

func (c *Cache[K, V]) Remove(key K) {
	c.cache.Remove(key)
}

// Insert will add a resource directly to the cache with the given ID
func (c *Cache[K, V]) Insert(key K, resource V) {
	// cache the resource and eject least recently used one if we reached limit
	evicted := c.cache.Add(key, resource)
	if !evicted {
		c.metrics.CacheEntries(c.resource, uint(c.cache.Len()))
	}
}

// PutTx will return tx which adds a resource to the cache with the given ID.
func (c *Cache[K, V]) PutTx(key K, resource V) func(storage.BaseReaderBatchWriter) error {
	storeOps := c.store(key, resource) // assemble DB operations to store resource (no execution)

	return func(rw storage.BaseReaderBatchWriter) error {
		storage.OnCommitSucceed(rw, func() {
			c.Insert(key, resource)
		})

		err := storeOps(rw) // execute operations to store resource
		if err != nil {
			return fmt.Errorf("could not store resource: %w", err)
		}

		return nil
	}
}
