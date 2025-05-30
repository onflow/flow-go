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

type storeFunc[K comparable, V any] func(rw storage.ReaderBatchWriter, key K, val V) error

const DefaultCacheSize = uint(1000)

// nolint:unused
func withStore[K comparable, V any](store storeFunc[K, V]) func(*Cache[K, V]) {
	return func(c *Cache[K, V]) {
		c.store = store
	}
}

// nolint:unused
func noStore[K comparable, V any](_ storage.ReaderBatchWriter, _ K, _ V) error {
	return fmt.Errorf("no store function for cache put available")
}

// nolint: unused
func noopStore[K comparable, V any](_ storage.ReaderBatchWriter, _ K, _ V) error {
	return nil
}

type removeFunc[K comparable] func(storage.ReaderBatchWriter, K) error

func withRemove[K comparable, V any](remove removeFunc[K]) func(*Cache[K, V]) {
	return func(c *Cache[K, V]) {
		c.remove = remove
	}
}

func noRemove[K comparable](_ storage.ReaderBatchWriter, _ K) error {
	return fmt.Errorf("no remove function for cache remove available")
}

type retrieveFunc[K comparable, V any] func(r storage.Reader, key K) (V, error)

// nolint:unused
func withRetrieve[K comparable, V any](retrieve retrieveFunc[K, V]) func(*Cache[K, V]) {
	return func(c *Cache[K, V]) {
		c.retrieve = retrieve
	}
}

// nolint:unused
func noRetrieve[K comparable, V any](_ storage.Reader, _ K) (V, error) {
	var nullV V
	return nullV, fmt.Errorf("no retrieve function for cache get available")
}

type Cache[K comparable, V any] struct {
	metrics module.CacheMetrics
	// nolint:unused
	limit    uint
	store    storeFunc[K, V]
	retrieve retrieveFunc[K, V]
	remove   removeFunc[K]
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
		remove:   noRemove[K],
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
func (c *Cache[K, V]) Get(r storage.Reader, key K) (V, error) {
	// check if we have it in the cache
	resource, cached := c.cache.Get(key)
	if cached {
		c.metrics.CacheHit(c.resource)
		return resource, nil
	}

	// get it from the database
	resource, err := c.retrieve(r, key)
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
func (c *Cache[K, V]) PutTx(rw storage.ReaderBatchWriter, key K, resource V) error {
	storage.OnCommitSucceed(rw, func() {
		c.Insert(key, resource)
	})

	err := c.store(rw, key, resource)
	if err != nil {
		return fmt.Errorf("could not store resource: %w", err)
	}

	return nil
}

func (c *Cache[K, V]) RemoveTx(rw storage.ReaderBatchWriter, key K) error {
	storage.OnCommitSucceed(rw, func() {
		c.Remove(key)
	})

	err := c.remove(rw, key)
	if err != nil {
		return fmt.Errorf("could not remove resource: %w", err)
	}

	return nil
}

func (c *Cache[K, V]) RemoveFunc(del func(key K) bool) {
	_ = c.cache.RemoveFunc(del)
}
