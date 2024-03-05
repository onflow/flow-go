package pebble

import (
	"errors"
	"fmt"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

const DefaultCacheSize = uint(10_000)

type CacheType int

const (
	CacheTypeLRU CacheType = iota + 1
	CacheTypeARC
	CacheTypeTwoQueue
)

func ParseCacheType(s string) (CacheType, error) {
	switch s {
	case CacheTypeLRU.String():
		return CacheTypeLRU, nil
	case CacheTypeARC.String():
		return CacheTypeARC, nil
	case CacheTypeTwoQueue.String():
		return CacheTypeTwoQueue, nil
	default:
		return 0, errors.New("invalid cache type")
	}
}

func (m CacheType) String() string {
	switch m {
	case CacheTypeLRU:
		return "lru"
	case CacheTypeARC:
		return "arc"
	case CacheTypeTwoQueue:
		return "2q"
	default:
		return ""
	}
}

type CacheBackend interface {
	Get(key string) (value flow.RegisterValue, ok bool)
	Add(key string, value flow.RegisterValue)
	Contains(key string) bool
	Len() int
	Remove(key string)
}

// wrapped is a wrapper around lru.Cache to implement CacheBackend
// this is needed because the standard lru cache implementation provides additional features that
// the ARC and 2Q caches do not. This standardizes the interface to allow swapping between types.
type wrapped struct {
	cache *lru.Cache[string, flow.RegisterValue]
}

func (c *wrapped) Get(key string) (value flow.RegisterValue, ok bool) {
	return c.cache.Get(key)
}
func (c *wrapped) Add(key string, value flow.RegisterValue) {
	_ = c.cache.Add(key, value)
}
func (c *wrapped) Contains(key string) bool {
	return c.cache.Contains(key)
}
func (c *wrapped) Len() int {
	return c.cache.Len()
}
func (c *wrapped) Remove(key string) {
	_ = c.cache.Remove(key)
}

type ReadCache struct {
	metrics  module.CacheMetrics
	resource string
	cache    CacheBackend
	retrieve func(key string) (flow.RegisterValue, error)
}

func newReadCache(
	collector module.CacheMetrics,
	resourceName string,
	cacheType CacheType,
	cacheSize uint,
	retrieve func(key string) (flow.RegisterValue, error),
) (*ReadCache, error) {
	cache, err := getCache(cacheType, int(cacheSize))
	if err != nil {
		return nil, fmt.Errorf("could not create cache: %w", err)
	}

	c := ReadCache{
		metrics:  collector,
		resource: resourceName,
		cache:    cache,
		retrieve: retrieve,
	}
	c.metrics.CacheEntries(c.resource, uint(c.cache.Len()))

	return &c, nil
}

func getCache(cacheType CacheType, size int) (CacheBackend, error) {
	switch cacheType {
	case CacheTypeLRU:
		cache, err := lru.New[string, flow.RegisterValue](size)
		if err != nil {
			return nil, err
		}
		return &wrapped{cache: cache}, nil
	case CacheTypeARC:
		return lru.NewARC[string, flow.RegisterValue](size)
	case CacheTypeTwoQueue:
		return lru.New2Q[string, flow.RegisterValue](size)
	default:
		return nil, fmt.Errorf("unknown cache type: %d", cacheType)
	}
}

// IsCached returns true if the key exists in the cache.
// It DOES NOT check whether the key exists in the underlying data store.
func (c *ReadCache) IsCached(key string) bool {
	return c.cache.Contains(key)
}

// Get will try to retrieve the resource from cache first, and then from the
// injected. During normal operations, the following error returns are expected:
//   - `storage.ErrNotFound` if key is unknown.
func (c *ReadCache) Get(key string) (flow.RegisterValue, error) {
	resource, cached := c.cache.Get(key)
	if cached {
		c.metrics.CacheHit(c.resource)
		if resource == nil {
			return nil, storage.ErrNotFound
		}
		return resource, nil
	}

	// get it from the database
	resource, err := c.retrieve(key)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.cache.Add(key, nil)
			c.metrics.CacheEntries(c.resource, uint(c.cache.Len()))
			c.metrics.CacheNotFound(c.resource)
		}
		return nil, fmt.Errorf("could not retrieve resource: %w", err)
	}

	c.metrics.CacheMiss(c.resource)

	c.cache.Add(key, resource)
	c.metrics.CacheEntries(c.resource, uint(c.cache.Len()))

	return resource, nil
}

func (c *ReadCache) Remove(key string) {
	c.cache.Remove(key)
}

// Insert will add a resource directly to the cache with the given ID
func (c *ReadCache) Insert(key string, resource flow.RegisterValue) {
	c.cache.Add(key, resource)
	c.metrics.CacheEntries(c.resource, uint(c.cache.Len()))
}
